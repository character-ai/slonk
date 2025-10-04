#!/usr/bin/env python3

import logging
from slonk.utils import bash

logger = logging.getLogger(__name__)

THRESHOLD = 20  # in cores


def cpu_load():
    uptime = bash("uptime").strip()
    # example:
    # 20:46:56 up 19 days, 19:41,  0 users,  load average: 75.23, 74.72, 73.71
    load = max(float(x) for x in uptime.split("load average: ")[-1].split(", "))
    if load > THRESHOLD:
        logger.warning(f"CPU load is high: {uptime}")


def all_checks():
    cpu_load()
