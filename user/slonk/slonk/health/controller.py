#!/usr/bin/env python3

"""
A health check for checking reachability of the control plane.
"""

import logging
import random

from slonk.health.base import CalledProcessError, NeedsManual
from slonk.utils import bash, get_cluster

logger = logging.getLogger(__name__)

# very high tolerance for failure
ATTEMPTS = 10
CONTROL_PLANES = os.environ.get("CONTROL_PLANE_HOSTS", "").split(",") if os.environ.get("CONTROL_PLANE_HOSTS") else []


def ping_check():
    if not CONTROL_PLANES or not CONTROL_PLANES[0]:
        logger.info("No control plane hosts configured, skipping ping check")
        return

    for _ in range(ATTEMPTS):
        try:
            cp = random.choice(CONTROL_PLANES)
            bash(f"ping -c 1 {cp}")
            break
        except CalledProcessError:
            pass
    else:
        raise NeedsManual(
            "PingCheck", "PingCheckFailure", "Could not reach control plane"
        )


def all_checks():
    return
    if get_cluster() != "cluster1":
        return
    ping_check()
