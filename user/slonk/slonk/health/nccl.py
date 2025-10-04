#!/usr/bin/env python3

import logging
import os
import contextlib

from slonk.health.base import CalledProcessError, NeedsManual
from slonk.utils import (
    bash,
    get_cluster,
    is_a100_cluster,
    is_h100_cluster,
    is_tcpx_cluster,
    do_gpus_exist,
)


logger = logging.getLogger(__name__)

BIN = "/usr/local/bin/all_reduce_perf"


def _tcpx_config_for_version(version_string):
    _LD_LIBRARY_PATH = os.environ.get("LD_LIBRARY_PATH", "")
    base_config = {
        ## The next config is very unlikely to need to be set.
        #
        # Which interface should be used by the default NCCL plugin. This plugin uses raw TCP socket for things like
        # early session establishment. Performance is pretty terrible and it's only used during setup, so default to
        # the NIC directly connected to the CPU.
        "NCCL_SOCKET_IFNAME": "eth0",
        # CPU pinning for polling threads. eth{1,2} -> Socket0, eth{3,4} -> Socket1
        "NCCL_GPUDIRECTTCPX_TX_BINDINGS": "eth1:8-21,112-125;eth2:8-21,112-125;eth3:60-73,164-177;eth4:60-73,164-177",
        "NCCL_GPUDIRECTTCPX_RX_BINDINGS": "eth1:22-35,126-139;eth2:22-35,126-139;eth3:74-87,178-191;eth4:74-87,178-191",
        # double comment: trying out removing eth0 as ctrl dev
        # # eth0 is directly connected to Socket0, so we use it as the control interface.
        # # "NCCL_GPUDIRECTTCPX_CTRL_DEV": "eth0",
        # eth{1,4} are the network cards co-located with 2 gpus under the same PCIe switch.
        "NCCL_GPUDIRECTTCPX_SOCKET_IFNAME": "eth1,eth2,eth3,eth4",
        # Instruct to use TCPDirect when the GPU & NIC are under the same switch. The flag originally controls
        # when to use GPUDirect, but tcpx yoinks it.
        "NCCL_NET_GDR_LEVEL": "PIX",
        # Whether NCCL rings are allowed to use diferent NICs in different machines. On GCP each of the NIC forms a
        # standalone "bag-on-the-side" network. To protect against cross-rail traffic, traffic is not routed between
        # rails[1]. This means that enabling x-nic w/ tcpx will result in hangs.
        #
        # [1] - To check this, ssh into two machines under the same superblock. For example, cluster-h100-12-7{8,9},
        #       and try to traceroute between different NICs:
        # cluster-h100-12-79 $ ip a s eth2
        #   inet 10.136.0.226/32 metric 1024 scope global dynamic eth2
        # cluster-h100-12-79 $ ip a s eth1
        #    inet 10.128.1.19/32 metric 1024 scope global dynamic eth1
        #
        # cluster-h100-12-78 $ traceroute -i eth2 10.128.2.18
        #   traceroute to 10.128.2.18 (10.128.2.18), 30 hops max, 60 byte packets
        #    1  * * *
        #   ...
        # cluster-h100-12-78 $ traceroute -i eth2 10.136.0.226
        #   traceroute to 10.136.0.226 (10.136.0.226), 30 hops max, 60 byte packets
        #    1  10.136.0.226 (10.136.0.226)  1.148 ms * *
        "NCCL_CROSS_NIC": 0,
        # RING uses simple ring allreduces, which is bandwidth optimal for non-blocking networks.
        "NCCL_ALGO": "RING",
        # Wire protocol used by NCCL to transport data. Options are:
        #   * Simple: Send data eagerly, fence, completion. This option is able to utilize nearly
        #       100% of link bw, but has higher latency due to the fence and suffers from poor
        #       utilization at low transfer sizes.
        #   * LL{,128}: Nearly the same protocol, LL/LL128 transfers rely on atomic 8B/128B atomic
        #       ordered operations. Both protocols use the first 4B as a control flag, which means
        #       these protocols cap at 50/95% of link utilization. For more information, refer to
        #       this bug: https://github.com/NVIDIA/nccl/issues/281#issuecomment-571816990.
        "NCCL_PROTO": "Simple",
        # Google patched NCCL internally and introduced a knob that _looks_ like a NCCL flag but
        # isn't. Lovely. Probably better not to touch unless we're told to.
        "NCCL_DYNAMIC_CHUNK_SIZE": 524288,
        # Path to the tcpx unix socket used to communicate with the sidecar. Must be kept in sync
        # with nodetools.yaml.
        "NCCL_GPUDIRECTTCPX_UNIX_CLIENT_PREFIX": "/run/tcpx",
        # Enable multipath routing. When creating a connection to a host B, instead of opening a
        # single collection, TCPX will open K connections and route them based on smallest queue.
        "NCCL_GPUDIRECTTCPX_SCHED_ALG": "KATY",
        ## The arguments below this line likely need some work.
        #
        # GPUDirect mystery flags. yessir.
        "NCCL_GPUDIRECTTCPX_PROGRAM_FLOW_STEERING_WAIT_MICROS": 1000000,
        # NVLINK Sharp isn't supported on some deployments. We need to disable it from
        # NCCL 2.20+.
        "NCCL_NVLS_ENABLE": 0,
        # Verbosity logs.
        # TODO: We should bump this *at least* to WARN once the logspam issue is fixed.
        "NCCL_DEBUG": "INFO",
        "NCCL_DEBUG_SUBSYS": "ENV",
        # These flags control the number of sockets/threads per socket of the default NCCL tcp
        # plugin. Unless we're disabling tcpx, we should not bother with these.
        # TODO: Figure out if we can just jettison these.
        "NCCL_SOCKET_NTHREADS": 1,
        "NCCL_NSOCKS_PERTHREAD": 4,
        # Whether NCCL should ignore socket affinity when defining the rings. It shouldn't matter
        # due to the restrictive NIC mapping, and I'm suspicious of this setting.
        # TODO: My gut feeling is that shouldn't be setting this. Investigate.
        "NCCL_IGNORE_CPU_AFFINITY": 1,
    }

    per_version_knobs = {
        "v1.3.7": {
            # TODO: These numbers came straight from Google. Run a parameter sweep to finetune them.
            "NCCL_BUFFSIZE": 4194304,
            "NCCL_P2P_NET_CHUNKSIZE": 524288,
            "NCCL_P2P_PCI_CHUNKSIZE": 524288,
            "NCCL_P2P_NVL_CHUNKSIZE": 1048576,
            "NCCL_MAX_NCHANNELS": 8,
            "NCCL_MIN_NCHANNELS": 8,
            # TODO: Google gave us a much smaller number, check if it's sane.
            "NCCL_GPUDIRECTTCPX_PROGRAM_FLOW_STEERING_WAIT_MICROS": 1000,
            # PXN[1] enables more efficient broadcast, and more efficient ring topologies in networks
            # that are not relevant to this deployment (e.g. 1:1 NIC:GPU w/ cross-nic comms). Google folks
            # recommended keeping it off, but if we benfit from a2a performance we should bug them about it.
            # TODO: Follow up on this.
            # [1] - https://developer.nvidia.com/blog/doubling-all2all-performance-with-nvidia-collective-communication-library-2-12/
            "NCCL_P2P_PXN_LEVEL": 0,
        }
    }

    if version_string in per_version_knobs:
        knobs = per_version_knobs[version_string]
        knobs["LD_LIBRARY_PATH"] = (
            f"/home/common/tcpx/{version_string}:{_LD_LIBRARY_PATH}",
        )
    else:
        # TODO: Raise an exception once we have ubiquitious versioning.
        knobs = {
            "NCCL_BUFFSIZE": 4194304,
            "NCCL_P2P_NET_CHUNKSIZE": 524288,
            "NCCL_P2P_PCI_CHUNKSIZE": 524288,
            "NCCL_P2P_NVL_CHUNKSIZE": 524288,
            "NCCL_MAX_NCHANNELS": 16,  # Google: 8
            "NCCL_MIN_NCHANNELS": 16,  # Google: 8
            "LD_LIBRARY_PATH": f"/home/common/tcpx:{_LD_LIBRARY_PATH}",
            # GPUDirect mystery flags. yessir.
            "NCCL_GPUDIRECTTCPX_PROGRAM_FLOW_STEERING_WAIT_MICROS": 1000000,
        }

    return base_config | knobs


def get_nccl_envvars():
    if get_cluster() == "cluster-a100":
        return {
            "NCCL_TOPO_FILE": "/home/common/ndv4-topo.xml",
            "NCCL_IB_PCI_RELAXED_ORDERING": 1,
            "UCX_IB_PCI_RELAXED_ORDERING": 1,
            "NCCL_SOCKET_IFNAME": "eth0",
            "UCX_NET_DEVICES": "eth0",
            "CUDA_DEVICE_ORDER": "PCI_BUS_ID",
            "OMPI_MCA_COLL_HCOLL_ENABLE": 0,
            "UCX_TLS": "tcp",
            "NCCL_IGNORE_CPU_AFFINITY": 1,
        }
    elif get_cluster() == "cluster-tcpx":
        # TCPX_VERSION is set by nodepools.yaml
        retval = _tcpx_config_for_version(os.environ.get("TCPX_VERSION", ""))
        # cpu nodes won't have this
        libnccl = "/home/common/tcpx/nccl/2.20.5-1+cuda12.4/libnccl.so"
        if os.path.exists(libnccl):
            retval["LD_PRELOAD"] = libnccl
        return retval
    elif get_cluster() == "cluster1":
        return {
            "NCCL_IB_HCA": "mlx5_0,mlx5_1,mlx5_2,mlx5_5,mlx5_6,mlx5_7,mlx5_8,mlx5_11",
            "OMPI_MCA_COLL_HCOLL_ENABLE": "0",
            "NCCL_ALGO": "NVLS",
            "NCCL_SOCKET_IFNAME": "eth0",
            "NCCL_COLLNET_ENABLE": "0",
            "LD_PRELOAD": "/home/common/nccl/nccl-2.19.3/libnccl.so.2.19.3",
            "NCCL_DEBUG": "WARN",
        }
    else:
        return {}


def get_nvlink_threshhold():
    if is_a100_cluster():
        return 23.0
    elif is_tcpx_cluster():
        # TODO: raise this when we resolve the nvlink issues
        return 300.0
    elif is_h100_cluster():
        return 455.0
    raise ValueError(f"Unknown cluster {get_cluster()}")


def get_loopback_threshhold():
    if get_cluster() == "cluster-a100":
        return 19.0
    elif get_cluster() == "cluster-tcpx":
        return -1
    elif get_cluster() == "cluster1":
        return 43
    elif get_cluster() == "cluster2":
        return -1
    elif get_cluster() == "cluster3":
        return -1
    raise ValueError(f"Unknown cluster {get_cluster()}")


def get_pairwise_threshhold():
    if get_cluster() == "cluster-a100":
        return 187.0
    elif get_cluster() == "cluster-tcpx":
        return -1
    elif get_cluster() == "cluster1":
        # TODO: this doesn't seem right, measure myself at some point
        return 45.0
    elif get_cluster() == "cluster2":
        return -1
    raise ValueError(f"Unknown cluster {get_cluster()}")


@contextlib.contextmanager
def checkpoint_environment(new_vars):
    original = os.environ.copy()
    for key in original.keys():
        if key.startswith("SLURM_") or key.startswith("SRUN_"):
            del os.environ[key]
    for key, value in new_vars.items():
        os.environ[key] = str(value)
    yield
    for key, _ in list(os.environ.items()):
        if key not in original:
            del os.environ[key]
        else:
            os.environ[key] = original[key]


def _run_singleton_test(test_name, nccl_vars, threshold, msg_size_gb=2):
    if threshold < 0:
        logger.info(f"Skipping {test_name} because no threshold is set.")
        return
    goal = str(int(msg_size_gb * 1024 * 1024 * 1024))
    logger.info(f"Starting {test_name} nccl test")
    logger.debug(f"NCCL env: {nccl_vars}")
    with checkpoint_environment(nccl_vars):
        try:
            lines = bash(
                f"{BIN} -g 8 -b {goal} -e {goal}", split_lines=True, silent_stderr=False
            )
            for line in lines:
                if goal in line and "nThread" not in line:
                    result = float(line.strip().split()[-2])
                    logger.info(f"Measured speed {result} GB/s")
                    if result < threshold:
                        msg = f"Measured {test_name} speed {result} GB/s < {threshold}"
                        logger.error(msg)
                        raise NeedsManual(
                            "NCCLCheck",
                            f"NCCLCheck{test_name.capitalize()}Failure",
                            msg,
                        )

                    break
        except CalledProcessError as cpe:
            logger.error(f"{test_name} nccl test failed:\nstdout:\n{cpe.stdout}\nstderr:{cpe.stderr}")
            raise NeedsManual(
                "NCCLCheck",
                "NCCLCheckFailure",
                f"{test_name} nccl test failed:\nstdout:\n{cpe.stdout}\nstderr:{cpe.stderr}",
            )


def fast_nvlink_check():
    # run a very fast nvlink just to ensure it works
    logger.info("Running fast nvlink test")
    cvd = os.environ.get("CUDA_VISIBLE_DEVICES")
    if cvd:
        if len(cvd.split(",")) != 8:
            logger.info(f"Skipping nccl tests because CVD={cvd}")
            return
    check_nvlink(msg_size_gb=0.01, threshold=0.1)


def check_nvlink(msg_size_gb=2, threshold=None):
    nccl_vars = get_nccl_envvars()
    _run_singleton_test(
        "NVLink",
        nccl_vars,
        threshold=threshold or get_nvlink_threshhold(),
        msg_size_gb=msg_size_gb,
    )


def check_loopback(msg_size_gb=2):
    nccl_vars = get_nccl_envvars()
    nccl_vars["NCCL_SHM_DISABLE"] = 1
    nccl_vars["NCCL_P2P_DISABLE"] = 1
    _run_singleton_test("loopback", nccl_vars, threshold=get_loopback_threshhold())


def all_checks():
    if not os.path.exists(BIN):
        logger.info("Skipping nccl tests")
        return
    if not do_gpus_exist():
        logger.info("Skipping htod because no GPUs exist")
        return

    check_nvlink()
    check_loopback()
    logger.info("NCCL tests pass")


_DEFAULT_CONFIG = {
    "NCCL_ALGO": "RING",
    "NCCL_PROTO": "Simple",
    "NCCL_SOCKET_NTHREADS": 1,
    "NCCL_NSOCKS_PERTHREAD": 4,
    "NCCL_CROSS_NIC": 0,
    "NCCL_DYNAMIC_CHUNK_SIZE": 524288,
    "NCCL_BUFFSIZE": 4194304,
    "NCCL_GPUDIRECTTCPX_SOCKET_IFNAME": "eth1,eth2,eth3,eth4",
    "NCCL_GPUDIRECTTCPX_CTRL_DEV": "eth0",
    "NCCL_NET_GDR_LEVEL": "PIX",
    "NCCL_GPUDIRECTTCPX_PROGRAM_FLOW_STEERING_WAIT_MICROS": 1000000,
    "NCCL_IGNORE_CPU_AFFINITY": 1,
    "NCCL_DEBUG": "VERSION",
    "LD_LIBRARY_PATH": f"/home/common/tcpx:{os.environ.get('LD_LIBRARY_PATH', '')}",
    "NCCL_P2P_NET_CHUNKSIZE": 524288,
    "NCCL_P2P_PCI_CHUNKSIZE": 524288,
    "NCCL_GPUDIRECTTCPX_UNIX_CLIENT_PREFIX": "/run/tcpx",
    "NCCL_P2P_NVL_CHUNKSIZE": 524288,
    "NCCL_GPUDIRECTTCPX_TX_BINDINGS": "eth1:8-21,112-125;eth2:8-21,112-125;eth3:60-73,164-177;eth4:60-73,164-177",
    "NCCL_GPUDIRECTTCPX_RX_BINDINGS": "eth1:22-35,124-139;eth2:22-35,124-139;eth3:74-87,178-191;eth4:74-87,178-191",
    "NCCL_SOCKET_IFNAME": "eth0",
    "NCCL_MAX_NCHANNELS": 16,
    "NCCL_MIN_NCHANNELS": 16,
}

if __name__ == "__main__":
    import cai_logging

    d = _DEFAULT_CONFIG
    v = _tcpx_config_for_version("")
    # TODO: Remove this once we migrate away from the default config.
    assert (
        d == v
    ), f"Mismatch:\n+{set(d.items())-set(v.items())}\n, -{set(v.items())-set(d.items())}"

    cai_logging.install(
        source="none",
    )
    fast_nvlink_check()
    # all_checks()
