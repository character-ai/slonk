#!/usr/bin/env python

import os
import json
import logging

from slonk.utils import bash, do_gpus_exist
from slonk.health.base import NeedsManual, CalledProcessError

logger = logging.getLogger(__name__)


def dcgmi_check(level=1):
    cvd = os.environ.get("CUDA_VISIBLE_DEVICES", "")
    if not cvd:
        logger.info(f"Skipping dcgmi because CUDA_VISIBLE_DEVICES={cvd}")
        return

    try:
        output = bash(f"dcgmi diag -r {level} -j -i {cvd}")
        will_fail = False
    except CalledProcessError as cpe:
        output = cpe.stdout
        will_fail = True

    all_results = json.loads(output)
    test_categories = all_results["DCGM GPU Diagnostic"]["test_categories"]
    for test_category in test_categories:
        category_name = test_category["category"]
        tests = test_category["tests"]
        for test in tests:
            testname = test["name"]
            for result in test["results"]:
                status = result["status"]
                if "Pass" not in status:
                    msg = (
                        f"Failed dcgmi -r {level} test {category_name} - "
                        f"{testname}: {result}"
                    )
                    logger.error(msg)
                    raise NeedsManual(
                        "DCGMICheck", f"DCGMI{category_name.capitalize()}Failure", msg
                    )
    if will_fail:
        msg = f"dcgmi diag -r {level} returned nonzero exit"
        logger.error(msg)
        raise NeedsManual("DCGMICheck", "DCGMICheckNoneZeroResult", msg)


def all_checks(fast: bool = True):
    logger.info(f"Beginning dcgmi check fast = {fast}")
    if not do_gpus_exist():
        logger.info("No gpus found. Skipping dcgmi tests")
        return
    if fast:
        level = 1
    else:
        level = 2
    dcgmi_check(level=level)
    logger.info(f"dcgmi level {level} tests pass")


if __name__ == "__main__":
    
    all_checks(fast=False)
