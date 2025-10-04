#!/usr/bin/env python3

import hashlib
import logging
from slonk.utils import bash, do_gpus_exist, NVIDIA_SMI_PATH
from slonk.health.base import CalledProcessError

logger = logging.getLogger(__name__)


def setup_args(parser):
    pass


def fingerprint() -> str:
    if not do_gpus_exist():
        return "unknown"

    try:
        output = bash(f"{NVIDIA_SMI_PATH} --query-gpu=uuid --format=csv,noheader")
        concat = output.replace("\n", "").encode("utf-8")
        return hashlib.sha256(concat).hexdigest()
    except CalledProcessError as cpe:
        logger.error(cpe)


def main(args):
    print(fingerprint())


if __name__ == "__main__":
    main()
