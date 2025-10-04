#!/usr/bin/env python3


# TODO:
# - clocks check


import os
import logging
import time
import xml.etree.ElementTree as ET

from slonk.health.base import (
    HealthCheckError,
    NeedsPodRestart,
    NeedsPowerCycle,
    NeedsManual,
    NeedsRMA,
    CalledProcessError,
)
from slonk.utils import bash, do_gpus_exist, NVIDIA_SMI_PATH, is_tcpx_cluster

logger = logging.getLogger(__name__)

# idle memory threshhold before we declare zombie
TCPX_IDLE_MEMORY_MB = 4700  # usually we measure 4616
INFB_IDLE_MEMORY_MB = 10  # usually we measure 4

# avoid race conditions in terminating jobs
MEMORY_CHECK_RETRIES = 3
MEMORY_WAIT_SEC = 5


def xml_to_dict(element):
    result = {}
    for child in element:
        key = child.tag
        if "id" in child.attrib:
            key += f"_{child.attrib['id'].replace(':', '')}"
        if len(child) == 0:
            result[key] = child.text
        else:
            result[key] = xml_to_dict(child)
            if "id" in child.attrib:
                result[key]["pci_id"] = child.attrib["id"]
    return result


def plain_nvidia_smi():
    logger.debug("Checking plan nvidia_smi")
    try:
        output = bash(NVIDIA_SMI_PATH)
        return output
    except CalledProcessError as cpe:
        output = cpe.stdout.strip()
        msg = f"nvidia-smi failed: {output}"
        logger.error(msg)
        if "Unable to determine the device handle" in output:
            raise NeedsPowerCycle(
                "NvidiaSMIPlainCheck", "NvidiaSMIPlainCheckNoDeviceHandle", msg
            )
        elif "fell off the bus" in output:
            raise NeedsPowerCycle(
                "NvidiaSMIPlainCheck", "NvidiaSMIPlainCheckFellOffBus", msg
            )
        else:
            raise NeedsManual("NvidiaSMIPlainCheck", "NvidiaSMIPlainCheckFailure", msg)


def parsed_nvidia_smi():
    if not os.environ.get("CUDA_VISIBLE_DEVICES"):
        logger.warning("Empty CUDA_VISIBLE_DEVICES, querying all GPUs")
        cvd = "0,1,2,3,4,5,6,7"
    else:
        cvd = os.environ["CUDA_VISIBLE_DEVICES"]
    logger.debug(f"Parsing nvidia-smi -q -x -i {cvd}")
    xml_str = bash(f"{NVIDIA_SMI_PATH} -q -x -i {cvd}")
    root = ET.fromstring(xml_str)
    parsed = xml_to_dict(root)
    gpus = [v for k, v in parsed.items() if k.startswith("gpu_")]
    for i, g in enumerate(gpus):
        g["id"] = i
        g["niceid"] = f"GPU#{id}"
    nongpus = {k: v for k, v in parsed.items() if not k.startswith("gpu_")}
    nongpus["gpus"] = gpus
    return nongpus


def mig(gpu):
    logger.debug("Checking for MIG")
    if gpu["mig_mode"]["current_mig"] != "Disabled":
        msg = "mig mode enabled for {gpu['niceid']}"
        logger.error(msg)
        raise NeedsManual("NvidiaSMIMIGCheck", "NvidiaSMIMIGEnabled", msg)


def needs_reset(gpu):
    logger.debug("Looking for 'GPU requires reset'")
    if gpu["remapped_rows"] == "GPU requires reset":
        msg = f"GPU Requires Reset on gpu {gpu['id']}"
        raise NeedsPowerCycle(
            "NvidiaSMIPlainCheck", "NvidiaSMIGpuRequiresResetFailure", msg
        )


def row_remap_pending(gpu):
    logger.debug("Checking for row remap pending")
    if gpu["remapped_rows"]["remapped_row_pending"] == "Yes":
        msg = f"Row remap pending on gpu {gpu['id']}"
        logger.error(msg)
        raise NeedsPowerCycle("NvidiaSMIRowRemapCheck", "NvidiaSMIRowRemapPending", msg)


def row_remap_failed(gpu):
    logger.debug("Checking for row remap failed")
    if gpu["remapped_rows"]["remapped_row_failure"] == "Yes":
        msg = f"Row remap FAILED on gpu {gpu['id']}. RMA!"
        logger.critical(msg)
        raise NeedsRMA("NvidiaSMIRowRemapCheck", "NvidiaSMIRowRemapFailed", msg)


def idle_memory(gpu, raw_output):
    logger.debug("Checkpoint for unexpected idle memory usage")
    memstr = gpu["fb_memory_usage"]["used"]
    assert "MiB" in memstr, "Got " + memstr
    mem_mb, _ = memstr.split()
    if is_tcpx_cluster():
        thresh = TCPX_IDLE_MEMORY_MB
    else:
        thresh = INFB_IDLE_MEMORY_MB
    if int(mem_mb) > thresh:
        msg = f"Unexpected memory usage on GPU {gpu['id']} ({memstr} used). output: {raw_output}"
        logger.error(msg)
        raise NeedsPodRestart(
            "NvidiaSMIIdleMemoryCheck", "NvidiaSMIIdleMemoryUnexpectedUsage", msg
        )


def check_gpu_count(gpus):
    logger.debug("Counting GPUs")
    if len(gpus) != 8:
        msg = f"Only has {len(gpus)} gpus!"
        raise NeedsPowerCycle("NvidiaSMIGPUCountCheck", "NvidiaSMIGPUMissing", msg)


def ecc(gpu, max_uncorrectable_ecc_errors=0, max_correctable_ecc_errors=25000):
    logger.debug("Checking for ECC errors")
    if gpu["ecc_mode"]["current_ecc"] != "Enabled":
        msg = "Unexpected ECC Mode!"
        logger.error(msg)
        raise NeedsManual("NvidiaSMIECCCheck", "NvidiaSMIECCModeError", msg)

    correctables = int(gpu["ecc_errors"]["volatile"]["dram_correctable"])
    uncorrectables = int(gpu["ecc_errors"]["volatile"]["dram_uncorrectable"])
    if correctables > max_correctable_ecc_errors:
        msg = "Too many DRAM ECC correctables (Got {correctables}, threshold {max_correctable_ecc_errors})"
        logger.error(msg)
        raise NeedsManual("NvidiaSMIECCCheck", "NvidiaSMIECCTooManyCorrectables", msg)
    if uncorrectables > max_uncorrectable_ecc_errors:
        msg = "Too many DRAM ECC uncorrectables (Got {uncorrectables}, threshold {max_uncorrectable_ecc_errors})"
        logger.error(msg)
        raise NeedsPowerCycle(
            "NvidiaSMIECCCheck", "NvidiaSMIECCTooManyUncorrectables", msg
        )


def all_checks():
    logger.debug("Running all nvidia-smi checks")
    if not do_gpus_exist():
        logger.info("No gpus found. Skipping nvidia-smi checks.")
        return
    raw_output = plain_nvidia_smi()

    parsed_output = parsed_nvidia_smi()
    for i, gpu in enumerate(parsed_output["gpus"]):
        logger.debug(f"Checking GPU {i}")
        needs_reset(gpu)
        row_remap_pending(gpu)
        row_remap_failed(gpu)
        mig(gpu)
        ecc(gpu)

    for attempt in range(1, MEMORY_CHECK_RETRIES + 1):
        # memory sometimes takes a tad to clean up, so add retries
        parsed_output = parsed_nvidia_smi()
        # check each gpu
        for i, gpu in enumerate(parsed_output["gpus"]):
            try:
                # will raise exception if there's memory in use
                idle_memory(gpu, raw_output)
            except NeedsPodRestart:
                # got an exception
                if attempt == MEMORY_CHECK_RETRIES:
                    # last attempt, escalate the error
                    logger.error(f"Still unexpected memory usage on gpu {i}. Raising!")
                    raise
                else:
                    # not last attempt
                    logger.warning(
                        f"Found memory on gpu {i} (attempt #{attempt}/{MEMORY_CHECK_RETRIES})"
                    )
                    break
        else:
            # no one complained (inner for loop didn't break), everyone is
            # clean. break out of the outer retry for loop
            break
        # someone complained, wait a second and try again
        time.sleep(MEMORY_WAIT_SEC)

    logger.info("nvidia-smi checks pass")


if __name__ == "__main__":
    import cai_logging

    
    all_checks()
