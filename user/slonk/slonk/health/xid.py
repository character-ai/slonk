#!/usr/bin/env python3

import logging
import re

from slonk.utils import bash
from slonk.health.base import NeedsManual

logger = logging.getLogger(__name__)

# ref: https://docs.nvidia.com/deploy/xid-errors/index.html
# recommended by msft:
# http://go/nhc/azure_gpu_xid.nhc#L4
# fmt: off
FATAL_XID = {
    48, 56, 57, 58, 62, 63, 64, 65, 68, 69, 73, 74, 79, 80, 81, 92, 119, 120,
    # cursed xid we found on some h100s
    # disabling for now
    109,
}
# fmt: on


def check_xid():
    """
    Looks for XID errors on the host.
    """
    count_13 = 0
    dmesg_lines = bash("dmesg -T", sudo=True, split_lines=True)
    for line in dmesg_lines:
        if "Xid" not in line:
            continue
        if m := re.search(r"Xid \(.*?\): (\d+), pid=.+?, (.*)$", line):
            xid_error, xid_msg = int(m[1]), m[2]
            if xid_error in FATAL_XID:
                msg = f"Fatal XID {xid_error}: {xid_msg}"
                logger.error(msg)
                raise NeedsManual("XidCheck", f"XidCheckError{xid_error}", msg)
            elif xid_error == 13:
                count_13 += 1

    msg = f"Found {count_13} Xid 13 errors"
    if count_13 > 1:
        logger.warning(msg)

    logger.info("ignoring non fatal XID checks for now!")


def all_checks():
    check_xid()


if __name__ == "__main__":
    check_xid()
