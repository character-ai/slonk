#!/usr/bin/env python

import logging
from slonk.utils import bash, get_cluster
from slonk.health.base import NeedsManual

logger = logging.getLogger(__name__)


PCI_BUSSES = {
    "18:00.0",
    "18:01.0",
    "18:02.0",
    "18:1f.0",
    "2b:00.0",
    "2b:01.0",
    "2b:02.0",
    "3e:00.0",
    "3e:01.0",
    "3e:02.0",
    "3e:1f.0",
    "64:00.0",
    "64:01.0",
    "64:02.0",
    "9a:00.0",
    "9a:01.0",
    "9a:02.0",
    "9a:1f.0",
    "ac:00.0",
    "ac:01.0",
    "ac:02.0",
    "be:00.0",
    "be:01.0",
    "be:02.0",
    "be:1f.0",
    "e2:00.0",
    "e2:01.0",
    "e2:02.0",
}


def lspci_check():
    last_device = None
    for line in bash("lspci -vvvv", sudo=True, split_lines=True):
        line = line.rstrip()
        if not line:
            continue
        if not line.startswith("\t"):
            # found a device
            last_device = line.split()[0]
        elif "ACSCtl:" in line:
            if "SrcValid-" in line and last_device in PCI_BUSSES:
                logger.debug(f"bus {last_device} is good")
            if "SrcValid+" in line and last_device in PCI_BUSSES:
                msg = f"PCI ACS is not set on bus {last_device}"
                logger.error(msg)
                raise NeedsManual("PCICheck", "PCIACSNotOnBus", msg)
    logger.info("lspci checks pass")


def all_checks():
    if get_cluster() != "cluster1":
        logger.info("Skipping PCI checks")
        return
    lspci_check()
