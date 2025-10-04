#!/usr/bin/env python3

from typing import Optional

import os
import logging
import socket
import subprocess
import urllib.request

logger = logging.getLogger(__name__)

POD_OVERRIDES = """
{
  "spec": {
    "hostPID": true,
    "hostNetwork": true,
    "tolerations": [
      {
        "operator": "Exists",
        "effect": "NoSchedule"
      },
      {
        "operator": "Exists",
        "effect": "NoExecute"
      }
    ],
    "containers": [
      {
        "name": "[[NAME]]",
        "image": "alpine",
        "command": [
          "/bin/sh"
        ],
        "args": [
          "-c",
          "[[COMMAND]]"
        ],
        "resources": null,
        "stdin": true,
        "stdinOnce": true,
        "terminationMessagePath": "/dev/termination-log",
        "terminationMessagePolicy": "File",
        "tty": true,
        "securityContext": {
          "privileged": true
        }
      }
    ],
    "nodeSelector": {
      "kubernetes.io/hostname": "[[NODE]]"
    }
  }
}
"""

if os.path.exists("/usr/local/nvidia/bin/nvidia-smi"):
    # hardcode path on gcp
    NVIDIA_SMI_PATH = "/usr/local/nvidia/bin/nvidia-smi"
else:
    NVIDIA_SMI_PATH = "nvidia-smi"


def get_cluster():
    return os.environ["K8S_CLUSTER_NAME"]


def is_onprem_cluster():
    return get_cluster() in {"cluster1", "cluster2", "cluster3"}


def is_tpu_cluster():
    return get_cluster().startswith("tpu")


def is_nvidia_cluster():
    return not is_tpu_cluster() and "-cpu" not in os.environ["K8S_NODE_NAME"]


def is_a100_cluster():
    return is_nvidia_cluster() and get_cluster() == "cluster-a100"


def is_h100_cluster():
    return is_nvidia_cluster() and not is_a100_cluster()


def is_tcpx_cluster():
    return get_cluster() in {"cluster-tcpx"}


def is_infiniband_cluster():
    return get_cluster() in {"cluster-a100", "cluster1", "cluster2", "cluster3"}


def is_pcie5_cluster():
    return get_cluster() in {"cluster1", "cluster2", "cluster3"}


def is_pcie4_cluster():
    return get_cluster() == "cluster-tcpx"


def gethostname():
    return socket.gethostname()


def do_gpus_exist():
    return any(os.path.exists(f"/dev/nvidia{i}") for i in range(8))


def curl(url, headers):
    request = urllib.request.Request(url, headers=headers)
    with urllib.request.urlopen(request) as response:
        return response.read().decode("ascii")


def bash(
    cmd,
    silent_stderr: bool = True,
    dry_run: bool = False,
    sudo: bool = False,
    split_lines: bool = False,
    timeout: Optional[int] = None,
):
    if dry_run:
        logger.info(f"Dry-run: Would have run '{cmd}'")
        return
    if sudo and os.environ.get("USER", "root") != "root":
        cmd = f"sudo {cmd}"

    stderr = subprocess.DEVNULL if silent_stderr else None
    logger.debug(f"Running command '{cmd}'")
    output = subprocess.check_output(
        cmd, shell=True, stderr=stderr, encoding="utf-8", timeout=timeout
    )
    logger.debug(f"Output from '{cmd}':\n{output}")
    if split_lines:
        return output.strip().split("\n")
    else:
        return output


def host_bash(
    node,
    cmd,
    silent_stderr: bool = True,
    dry_run: bool = False,
    split_lines: bool = False,
):
    raise NotImplementedError("doesn't work yet")
    name = f"hostexec-{hash(cmd)}"
    overrides = (
        POD_OVERRIDES.replace("[[COMMAND]]", cmd)
        .replace("[[NODE]]", node)
        .replace("[[NAME]]", name)
    )
    print(
        f"kubectl run {name} --attach --rm -q --image=alpine --overrides='{overrides}'",
    )
    return ""
    return bash(
        f"kubectl run {name} --attach --rm -q --image=alpine --overrides='{overrides}'",
        silent_stderr=False,
        dry_run=dry_run,
        split_lines=split_lines,
    )
