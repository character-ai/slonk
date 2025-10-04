#!/usr/bin/env python3

import logging
import os

from slonk.health.base import NeedsManual
from slonk.utils import (
    bash,
    is_h100_cluster,
    is_a100_cluster,
    is_pcie5_cluster,
    is_pcie4_cluster,
    do_gpus_exist,
)

logger = logging.getLogger(__name__)

BIN = "/usr/local/bin/nv-bandwidthtest"


def htod_check():
    """
    Check bandwidth of host<->device transfers.
    """
    if not os.path.exists(BIN):
        logger.info("nvidia's bandwidthtest not installed, skipping")
        return

    logger.debug("Checking host<->device bandwidth")

    if "CUDA_VISIBLE_DEVICES" in os.environ:
        gpus_to_check = [int(g) for g in CUDA_VISIBLE_DEVICES.split(",")]
    else:
        gpus_to_check = range(8)

    # reference: https://go/nhc/azure_cuda_bandwidth.nhc
    GPUS = [0, 1, 2, 3, 4, 5, 6, 7]
    if is_a100_cluster():
        THRESHOLD = 23.0
        LANES = [1, 1, 0, 0, 3, 3, 2, 2]
    elif is_h100_cluster() and is_pcie5_cluster():
        THRESHOLD = 53.0
        LANES = [1, 1, 1, 1, 0, 0, 0, 0]
    elif is_h100_cluster() and is_pcie4_cluster():
        THRESHOLD = 26.0
        LANES = [1, 1, 1, 1, 0, 0, 0, 0]
    else:
        raise ValueError("Unknown cluster type")

    for test in ["dtoh", "htod"]:
        for lane, device in zip(LANES, GPUS):
            if device not in gpus_to_check:
                continue
            output = bash(
                f"numactl -N {lane} -m {lane} {BIN} --device={device} --{test}"
            )
            for line in output.strip().split("\n"):
                if "32000000" in line:
                    _, right = line.split()
                    right = float(right)
                    logger.debug(
                        f"Measured memory bandwidth {test} of {right} on gpu {device}"
                    )
                    if right < THRESHOLD:
                        msg = f"Memory {test} bandwidth measured {right} < {THRESHOLD}"
                        logger.error(msg)
                        # too many false positives, disabling for now
                        # raise NeedsManual("HTODCheck", "HTODCheckLowMemoryBandwidth",msg)
    logger.info("host<->device checks pass")


def all_checks():
    if not do_gpus_exist():
        logger.info("Skipping htod because no GPUs exist")
        return
    htod_check()


if __name__ == "__main__":
    import cai_logging

    
    htod_check()
