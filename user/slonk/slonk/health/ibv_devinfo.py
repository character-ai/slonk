#!/usr/bin/env python3

import logging
from slonk.utils import get_cluster, bash

logger = logging.getLogger(__name__)

links_to_check = {
    "cluster-a100": [f"mlx5_{i}" for i in range(8)],
    "cluster1": [f"mlx5_{i}" for i in [0, 1, 2, 3, 5, 6, 7, 8, 11]],
    "cluster2": [f"mlx5_{i}" for i in [0, 3, 4, 5, 6, 9, 10, 11]],
}


def ibv_devinfo_check():
    logger.info("Checking ibv_devinfo")
    for device in links_to_check[get_cluster()]:
        for line in bash(f"ibv_devinfo -d {device}", split_lines=True):
            if "state:" in line:
                state = line.split()[1]
                if state != "PORT_ACTIVE":
                    logger.error(f"Invalid port state on {device}: {state}")


def all_checks():
    if get_cluster() not in links_to_check:
        logger.info("Skipping ibv_devinfo checks")
        return
    ibv_devinfo_check()
