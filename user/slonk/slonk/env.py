#!/usr/bin/env python3

import os
from slonk.fingerprint import fingerprint
from slonk.utils import bash, is_nvidia_cluster, NVIDIA_SMI_PATH


def setup_args(parser):
    pass


def concatenv(var, new):
    return new + ":" + os.environ.get(var, "")


def get_cuda_envs():
    if not is_nvidia_cluster():
        return {}
    if os.path.exists("/usr/local/cuda-12.2") or os.path.exists("/usr/local/cuda-12.3") or os.path.exists("/usr/local/cuda-12.4"):
        CUDA_HOME = "/usr/local/cuda"
        return {
            "CUDA_HOME": CUDA_HOME,
            "LD_LIBRARY_PATH": concatenv(
                "LD_LIBRARY_PATH",
                f"{CUDA_HOME}/lib64:{CUDA_HOME}/compat:{CUDA_HOME}/lib64/stubs:/usr/local/nvidia/lib64",
            ),
            "PATH": concatenv("PATH", f"{CUDA_HOME}/bin:/usr/local/nvidia/bin"),
            "_CUDA_COMPAT_PATH": f"{CUDA_HOME}/compat",
            "TRITON_PTXAS_PATH": f"{CUDA_HOME}/bin/ptxas",
            "ENABLE_TMA": "1",
            "ENABLE_MMA_V3": "1",
        }


def get_vars():
    # k8s vars and fingerprint
    out = {k: v for k, v in os.environ.items() if k.startswith("K8S_")}
    out["K8S_GPU_UUID_HASH"] = fingerprint()

    # preferences
    out["LC_TIME"] = "C.UTF-8"
    out["PDSH_RCMD_TYPE"] = "ssh"

    out = {**out, **get_cuda_envs()}

    out["PATH"] = (
        out.get("PATH", "") + ":/etc/slurm-cm-exe:/home/common/git-sync/k8s/k8s.git/bin"
    )
    return out


def main(args):
    for k, v in get_vars().items():
        print(f"{k}={v}")


if __name__ == "__main__":
    main()
