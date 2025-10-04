#!/usr/bin/env python3

"""
Double checks that this node is not part of Yiran's blacklist.
"""
import logging

from slonk.health.base import NeedsManual
from slonk.k8s import check_node_goal_state_okay

logger = logging.getLogger(__name__)


def all_checks():
    logger.info("Checking for node blacklist status")
    if not check_node_goal_state_okay():
        msg = "Node should have been blacklisted but is still in job"
        logger.error(msg)
        raise NeedsManual("SlurmGoalStateCheck", "SlurmGoalStateNotExpected", msg)
    logger.info("Node is okay to be part of slurm")


if __name__ == "__main__":
    import cai_logging

    
    all_checks()
