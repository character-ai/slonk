#!/usr/bin/env python3

import logging
import os


from slonk.fingerprint import fingerprint
import slonk.k8s as k8s
from slonk.utils import bash

logger = logging.getLogger(__name__)


def drain_slurm_node(reason):
    pod_name, _ = k8s._get_pod_name_and_namespace()
    physical_node_name = fingerprint()
    logger.info(f"Draining slurm node {pod_name} with reason {reason}, fingerprint: {physical_node_name}")

    k8s.update_physical_node_slurm_goal_state(physical_node_name, "drain", True, reason)
    logger.log(
        logging.INFO,
        f"Drained slurm node successfully, fingerprint: {physical_node_name}",
    )


def undrain_slurm_node():
    pod_name, _ = k8s._get_pod_name_and_namespace()
    physical_node_name = fingerprint()
    spec = k8s.get_physical_node_spec(physical_node_name)
    reason = spec.get("slurmNodeSpec", {}).get("reason", "")
    logger.info(f"Undraining slurm node {pod_name} with reason \"{reason}\", fingerprint: {physical_node_name}")

    k8s.update_physical_node_slurm_goal_state(physical_node_name, "up", False)
    bash(
        f"scontrol update node={pod_name} state=resume reason=",
        sudo=True
    )
    logger.log(
        logging.INFO,
        f"Undrained slurm node successfully, fingerprint: {physical_node_name}",
    )
