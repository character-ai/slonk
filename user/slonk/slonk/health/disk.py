#!/usr/bin/env python3


import logging

from slonk.utils import bash
from slonk.health.base import NeedsManual

logger = logging.getLogger(__name__)




def df_check():
    for line in bash("df -h", split_lines=True):
        if line.startswith("Filesystem"):
            continue
        fs, blocks, used, avail, usedpct, mount = line.split()
        if mount == "/mnt/localdisk" or mount == "/":
            usedpct = int(usedpct.rstrip("%"))
            if usedpct > 90:
                raise NeedsManual(
                    "DiskCheck",
                    "DiskAlmostFull",
                    f"Almost out of space on mount {mount}",
                )
    logger.info("Disk usage checks pass")


def all_checks():
    df_check()
