#!/usr/bin/env python3

import logging
import os
import sys
import socket
import time

import slonk.health
from slonk import k8s
from slonk import fingerprint
from slonk.utils import (
    bash,
    is_tpu_cluster,
    is_nvidia_cluster,
    is_tcpx_cluster,
    NVIDIA_SMI_PATH,
)
from slonk.health.base import HealthCheckError

from slonk.health.base import (
    NeedsPodRestart,
    CalledProcessError,
)

HF_DATASETS_CACHE = os.environ.get("HF_DATASETS_CACHE", "/home/common/hf_datasets_cache")

DUMMY_SLURM_JOBID = 99999

logger = logging.getLogger(__name__)


CAI_CORE_DUMP = os.environ.get("CAI_CORE_DUMP", "0") == "1"
CAI_CORE_DUMP_PATH = os.environ.get("CORE_DUMP_PATH", "/home/ephemeral/1d/coredumps")


def get_slurm_jobid():
    if "SLURM_JOBID" in os.environ:
        slurm_jobid = os.environ["SLURM_JOBID"]
    else:
        # In case prolog is invoked manually.
        slurm_jobid = DUMMY_SLURM_JOBID

    return slurm_jobid


def get_slurm_job_nodes():
    if "SLURM_JOB_NUM_NODES" in os.environ:
        return int(os.environ["SLURM_JOB_NUM_NODES"])
    else:
        # In case prolog is invoked manually.
        return 1


def get_slurm_tmpdir():
    if os.path.exists("/mnt/localdisk"):
        base = "/mnt/localdisk"
    else:
        base = "/tmp"

    slurm_jobid = get_slurm_jobid()

    return os.path.join(base, f"slurm_{slurm_jobid}")


def get_coredump_dir():
    return os.path.join(CAI_CORE_DUMP_PATH, get_slurm_jobid())


def rmrf(path, dry_run: bool = False):
    try:
        bash(
            f"find {path} -type f -print0 | xargs -0 --max-procs 16 rm -f",
            dry_run=dry_run,
            sudo=True,
        )
        bash(f"rm -rf {path}", dry_run=dry_run, sudo=True)
        logger.info(f"Successfully cleaned {path}")
    except CalledProcessError:
        logger.exception(f"Failed to clean {path}")


def kill_devices(devices, dry_run: bool = False):
    logger.debug("Killing processes using hardware devices")
    for device in devices:
        if not os.path.exists(f"/dev/{device}"):
            logger.debug(f"Skipping /dev/{device}")
            continue

        try:
            line = bash(f"lsof -w /dev/{device}", sudo=True).strip().split("\n")[-1]
        except CalledProcessError:
            logger.debug(f"No proceses using /dev/{device}")
            continue
        pid = line.split()[1]
        logger.info(f"Killing pid {pid} for using /dev/{device}")
        try:
            bash(f"kill -9 {pid}", dry_run=dry_run, sudo=True)
            bash(
                f"timeout 20 tail --pid={pid} -f /dev/null", dry_run=dry_run, sudo=True
            )
            logger.info(f"Successfully killed pid {pid} for using /dev/{device}")
        except CalledProcessError:
            logger.exception("Failed to kill pid {pid}!")
            raise NeedsPodRestart(
                "KillDevices", "KillDevicesFailure", "/dev/{device} is occupied"
            )


def _maybe_restart_tcpx(args):
    if not is_tcpx_cluster():
        # only restart tcpx on tcpx clusters
        return
    if os.environ.get("CUDA_VISIBLE_DEVICES") != "0,1,2,3,4,5,6,7":
        # not an 8gpu job, don't restart tcpx
        return

    logger.info("Restarting tcpx")
    bash(f"pkill -SIGINT tcpgpudmarxd", sudo=True, dry_run=args.dry_run)
    # empirically measured 2s for cleanup. give tcpx some time to clean up and restart
    time.sleep(5)


def kill_nvidia_procs(dry_run: bool = False):
    if "CUDA_VISIBLE_DEVICES" not in os.environ:
        logger.info("No CUDA_VISIBLE_DEVICES set, skipping killing procs.")
        return
    cvd = os.environ["CUDA_VISIBLE_DEVICES"]
    for line in bash(
        f"{NVIDIA_SMI_PATH} --query-compute-apps=gpu_bus_id,pid,used_memory,name --format=csv -i {cvd}",
        split_lines=True,
    ):
        if "pid" in line:
            continue
        busid, pid, mem, name = line.split(", ")
        if "tcpx" in name or "tcpgpudmarxd" in name:
            # don't kill tcpx
            continue
        logger.info(f"Killing pid {pid} for using GPU {busid}")
        try:
            bash(f"kill -9 {pid}", dry_run=dry_run, sudo=True)
            bash(
                f"timeout 30 tail --pid={pid} -f /dev/null", dry_run=dry_run, sudo=True
            )
            logger.info(f"Successfully killed pid {pid} for using GPU {busid}")
        except CalledProcessError:
            logger.exception(f"Failed to kill pid {pid}!")
            raise NeedsPodRestart(
                "KillNvidiaProc", "KillNvidiaProcFailure", f"GPU {busid} is occupied"
            )


def _both_prolog_and_epilog(args):
    if is_nvidia_cluster():
        logger.info("Nvidia cluster. Running nvidia prologs.")
        logger.info(f'CVD = {os.environ.get("CUDA_VISIBLE_DEVICES")}')
        slonk.health.run_fast_checks(args)
        kill_nvidia_procs(dry_run=args.dry_run)

    if is_tpu_cluster() and os.environ.get("TPU_ACCELERATOR_TYPE"):
        logger.info("TPU cluster. Running tpu prologs.")
        rmrf("/tmp/tpulogs/*", dry_run=args.dry_run)
        rmrf("/tmp/uscentral*", dry_run=args.dry_run)
        rmrf("/tmp/libtpu_lockfile", dry_run=args.dry_run)
        kill_devices(
            [f"accel{i}" for i in range(4)] + [f"vfio/{i}" for i in range(4)],
            dry_run=True,
        )


def _get_target_pod_deletion_cost() -> int:
    """
    Determine the deletion cost based on the job partition
    """
    partition_priorities = {
        "low": 10,
        "general": 100,
        "high": 1000,
        "dev": 10000,
        "priority": 100000,
    }

    job_partition = os.environ.get("SLURM_JOB_PARTITION", "general")
    if job_partition not in partition_priorities:
        logger.warning(
            "Missing tier for job partition %s. Defaulting to 'general' tier.",
            job_partition,
        )
        return partition_priorities["general"]
    return partition_priorities[job_partition]


def hf_dataset_cache_prolog():
    # This causes permissions errors
    bash(f"mkdir -p {HF_DATASETS_CACHE}", sudo=True)
    bash(f"chmod a+rwx {HF_DATASETS_CACHE}", sudo=True)
    bash(f"chown root:users {HF_DATASETS_CACHE}", sudo=True)
    bash(f"chmod g+s {HF_DATASETS_CACHE}", sudo=True)


def prolog(args):
    logger.info("Running prolog")
    logger.info("Making temp dir")
    bash(f"mkdir -p {get_slurm_tmpdir()}", sudo=True, dry_run=args.dry_run)
    bash(f"chmod a+rwx {get_slurm_tmpdir()}", sudo=True, dry_run=args.dry_run)

    slonk.health.run_lifecycle_checks(args)

    _maybe_restart_tcpx(args)
    _both_prolog_and_epilog(args)

    # Do this last to increase the chance of winning the race against epilog.
    # (By default, prolog and epilog could run in parallel.)
    pod_deletion_cost = _get_target_pod_deletion_cost()
    k8s.update_pod_deletion_cost(pod_deletion_cost)

    # Add monitoring label for job > 9 nodes
    if get_slurm_job_nodes() > 9:
        job_id = get_slurm_jobid()
        k8s.add_pod_label_slurm_jobid(job_id)
        k8s.add_node_label_slurm_jobid(job_id)

    logger.log(logging.INFO, "Successfully completed prolog")


def epilog(args):
    logger.info("Running epilog")
    # Do this first to increase the chance of losing the race against prolog.
    # (By default, prolog and epilog could run in parallel.)
    k8s.update_pod_deletion_cost(0)

    # Remove monitoring label if needed.
    if k8s.pod_slurm_jobid_label_exists():
        k8s.remove_pod_label_slurm_jobid()

    if k8s.node_slurm_jobid_label_exists():
        k8s.remove_node_label_slurm_jobid()

    try:
        rmrf(get_slurm_tmpdir(), dry_run=args.dry_run)
    except Exception:
        # log the exception but don't fail hard
        logger.exception(f"Failed to clean up slurm tmpdir {get_slurm_tmpdir()}")

    try:
        rmrf("/var/log/slurm-joblog/*", dry_run=args.dry_run)
    except Exception:
        # log the exception but don't fail hard
        logger.exception(f"Failed to clean up /var/log/slurm-joblog/*")

    _both_prolog_and_epilog(args)
    _maybe_restart_tcpx(args)

    logger.log(logging.INFO, "Successfully completed epilog")


def _set_envvar_in_task_prolog(name, value):
    line = f"export {name}={value}"
    logger.info(f"Outputting task prolog '{line}'")
    os.environ[name] = str(value)
    print(line)


def task_prolog(args):
    """
    Note that stdout created by the task prolog is interpretted by slurm to set
    env vars.

    Prints here are intentional, and the only place they should exist
    """
    logger.info("Running task prolog")
    logger.info("Turning off core dumps")
    bash("ulimit -c 0", dry_run=args.dry_run)

    logger.info("Setting k8s vars")
    _set_envvar_in_task_prolog("K8S_NODE_UUID", fingerprint.fingerprint())
    with open("/etc/profile.d/k8s_env.sh") as f:
        for line in f:
            if line.startswith("export "):
                line = line.strip().replace('"', "")
                logger.info(f"Outputting task prolog '{line}'")
                print(line)
                if line.startswith("K8S_MEM_REQUEST"):
                    limit_bytes = int(line.strip().split("\n"))
                    ulimit_bytes = int(0.99 * limit_bytes)
                    ulimit_kb = ulimit_bytes // 1024
                    logger.info(f"Setting ulimit -v {ulimit_kb}")
                    # bash(f"ulimit -v {ulimit_kb}", dry_run=args.dry_run)

    _set_envvar_in_task_prolog("SLURM_TMPDIR", get_slurm_tmpdir())
    _set_envvar_in_task_prolog("TMPDIR", get_slurm_tmpdir())

    # set a bunch of envvars for consistency across job launchers
    _set_envvar_in_task_prolog("TRITON_CACHE_DIR", get_slurm_tmpdir() + "/triton")
    _set_envvar_in_task_prolog("GRPC_ARG_HTTP2_MAX_PINGS_WITHOUT_DATA", 0)
    _set_envvar_in_task_prolog("RCLONE_FAST_LIST", 1)
    _set_envvar_in_task_prolog("RCLONE_HTTP_NO_HEAD", 1)
    _set_envvar_in_task_prolog("LC_TIME", "C.UTF-8")
    _set_envvar_in_task_prolog("DISABLE_COLORS", "1")

    # TODO: Dedupe this.
    _set_envvar_in_task_prolog("TCPX_VERSION", "v3.1.9-2.19.4-12.0")

    # TODO: Remove once debugserver has been validated.
    _set_envvar_in_task_prolog("ENABLE_DEBUGZ2", "YES")

    if k8s.get_pod_deletion_cost() == 0:
        pod_deletion_cost = _get_target_pod_deletion_cost()
        k8s.update_pod_deletion_cost(pod_deletion_cost)
        logger.info(
            "Pod deletion cost is unexpectedly 0, setting to %s in task-prolog",
            pod_deletion_cost,
        )

    if get_slurm_job_nodes() > 9 and not k8s.pod_slurm_jobid_label_exists():
        logger.info("Pod slurm jobid label is unexpectedly missing, adding it")
        k8s.add_pod_label_slurm_jobid(get_slurm_jobid())

    if get_slurm_job_nodes() > 9 and not k8s.node_slurm_jobid_label_exists():
        logger.info("Node slurm jobid label is unexpectedly missing, adding it")
        k8s.add_node_label_slurm_jobid(get_slurm_jobid())

    user = os.environ.get("SLURM_JOB_USER", "unknown_user")
    wandb_dir = f"/home/ephemeral/1d/{user}/slurm_{get_slurm_jobid()}/wandb"
    os.makedirs(wandb_dir, mode=0o777, exist_ok=True)
    _set_envvar_in_task_prolog("WANDB_DIR", wandb_dir)

    nccl_vars = slonk.health.nccl.get_nccl_envvars()
    for key, value in nccl_vars.items():
        if os.environ.get(key, value) != value:
            logger.warning(
                    f"Overriding {key}={value}. "
                    f"Old value: {os.environ[key]}"
            )
        _set_envvar_in_task_prolog(key, value)

    # finally set nccl logs for job logs
    jobid = get_slurm_jobid()
    restart_count = os.environ.get("SLURM_RESTART_COUNT", "0")
    rank = os.environ.get("SLURM_PROCID", "0")
    _set_envvar_in_task_prolog(
        "NCCL_DEBUG_FILE",
        f"/var/log/slurm-joblog/nccl_job{jobid}_restart{restart_count}_rank{rank}.log",
    )

    logger.log(logging.INFO, "Successfully completed task-prolog")


def task_epilog(args):
    logger.debug("Running task epilog")


def setup_args(parser):
    parser.add_argument(
        "--mode",
        choices={
            # compatible with `SLURM_SCRIPT_CONTEXT`,
            # so that we can do `slonk prolog --mode $SLURM_SCRIPT_CONTEXT`
            "prolog_slurmd",
            "epilog_slurmd",
            "prolog_task",
            "epilog_task",
            # legacy ones
            "prolog",
            "epilog",
            "task_prolog",
            "task_epilog",
        },
        required=True,
    )
    parser.add_argument(
        "-n",
        "--dry-run",
        action="store_true",
    )


def main(args):
    logger.info("Entering slurm checks")

    # Map legacy args.
    alias_mapping = {
        "prolog": "prolog_slurmd",
        "epilog": "epilog_slurmd",
        "task_prolog": "prolog_task",
        "task_epilog": "epilog_task",
    }
    mode = alias_mapping.get(args.mode, args.mode)

    try:
        if mode == "prolog_slurmd":
            prolog(args)
        elif mode == "epilog_slurmd":
            epilog(args)
        elif mode == "prolog_task":
            task_prolog(args)
        elif mode == "epilog_task":
            task_epilog(args)
        else:
            logger.error(f"Invalid mode: {args.mode}")
            raise ValueError(f"Invalid mode: {args.mode}")
    except HealthCheckError as hce:
        logger.critical(f"Failed health checks: {hce} ({hce.__class__.__name__})")
        if hce.handle_mitigation():
            sys.exit(0)
        else:
            logger.exception("Mitigation invoked and returned false. Exiting.")
            sys.exit(1)
    except Exception:
        logger.exception("Unknown exception")
        # intentionally don't exit nonzero when we have an unknown exception,
        # since 99% of the time there is a developer mistake
