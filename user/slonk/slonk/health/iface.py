#!/usr/bin/env python3

import logging
import time

from slonk.utils import bash

from slonk.health.base import NeedsManual

logger = logging.getLogger(__name__)

NUM_TOGGLE_TRIES = 5


def toggle_link(adapter):
    bash(f"ip link set {adapter} down", sudo=True)
    time.sleep(1)
    bash(f"ip link set {adapter} up", sudo=True)
    time.sleep(0.5)


def _iface_checks(toggle_on_missing: bool = True):
    device_count = 0
    inet_count = 0
    for line in bash("ifconfig | grep ^eth -A 1", split_lines=True):
        line = line.strip()
        if line.startswith("eth"):
            device = line.split()[0]
            device_count += 1
        elif line.startswith("--"):
            continue
        elif line.startswith("inet "):
            inet_count += 1
        else:
            if toggle_on_missing:
                logger.warning(f"Missing ipv4 address on {device}, toggling link!")
                toggle_link(device)
                return False
    if device_count != inet_count:
        msg = "Not enough ipv4 addresses"
        logger.error(msg)
        raise NeedsManual("IfaceCheck", "IfaceCheckFailure", msg)
    return True


def iface_checks():
    for attempt in range(1, NUM_TOGGLE_TRIES + 1):
        logger.info(f"starting iface check (attempt #{attempt}/{NUM_TOGGLE_TRIES})")
        should_toggle = attempt != NUM_TOGGLE_TRIES
        if _iface_checks():
            break
    logger.info("iface checks pass")


def all_checks():
    iface_checks()


if __name__ == "__main__":
    main()
