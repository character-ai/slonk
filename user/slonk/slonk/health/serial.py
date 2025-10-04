#!/usr/bin/env python3

"""
Serial number denylist
"""

import logging
from slonk.utils import bash

from slonk.health.base import (
    NeedsManual,
)


logger = logging.getLogger(__name__)

DENYLIST = {
    # placeholder just for syntactic example
    "__PLACEHOLDER__",
    # Add your banned serial numbers here
}


def check_serial_number():
    # serial = bash("dmidecode -s system-serial-number", sudo=True)
    serial = "not_implemented"
    logger.debug(f"Chassis serial number: {serial}")
    if serial in DENYLIST:
        msg = f"Found banned serial number {serial}"
        logger.error(msg)
        raise NeedsManual("SerialCheck", "SerialCheckFoundBannedSerialNumber", msg)


def all_checks():
    check_serial_number()


if __name__ == "__main__":
    main()
