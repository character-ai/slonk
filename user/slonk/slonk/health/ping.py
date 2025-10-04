#!/usr/bin/env python3

import json
import logging
import random

from slonk.utils import is_tcpx_cluster, bash
from slonk.health.base import NeedsPowerCycle, CalledProcessError

logger = logging.getLogger(__name__)

FAILURE_THRESHOLD = 0.5  # as a fraction of 1
NUM_PING_TRIES = 10
PING_TIMEOUT_MS = 100  # in milliseconds. typical success ping is <1ms
ADAPTERS = ["eth1", "eth2", "eth3", "eth4"]


class GracefulFallback(Exception):
    pass


def _find_nodes_in_my_superblock(remove_self=False):
    # avoid circular import
    import slonk.k8s as k8s

    nodes = k8s.list_nodes()
    my_node = k8s.get_my_node()

    try:
        my_superblock = my_node.metadata.labels["cloud.google.com/gke-placement-group"]
    except KeyError:
        raise GracefulFallback("Could not find my superblock")

    superblock = [
        n
        for n in nodes
        if n.metadata.labels.get("cloud.google.com/gke-placement-group")
        == my_superblock
    ]
    logger.info(f"Found {len(superblock)} nodes in my superblock")

    if remove_self:
        self = [n for n in superblock if n.metadata.name == my_node.metadata.name]
        superblock = [n for n in superblock if n.metadata.name != my_node.metadata.name]

    return superblock


def _get_adapter_ip(adapter, node):
    # example:
    # [
    #   {
    #     "birthIP": "10.0.1.201",
    #     "pciAddress": "0000:00:0c.0",
    #     "birthName": "eth0"
    #   },
    #   {
    #     "birthIP": "10.128.1.194",
    #     "pciAddress": "0000:06:00.0",
    #     "birthName": "eth1"
    #   },
    #   {
    #     "birthIP": "10.136.1.194",
    #     "pciAddress": "0000:0c:00.0",
    #     "birthName": "eth2"
    #   },
    #   {
    #     "birthIP": "10.144.1.194",
    #     "pciAddress": "0000:86:00.0",
    #     "birthName": "eth3"
    #   },
    #   {
    #     "birthIP": "10.152.1.194",
    #     "pciAddress": "0000:8c:00.0",
    #     "birthName": "eth4"
    #   }
    # ]
    if "networking.gke.io/nic-info" in node.metadata.annotations:
        nics = json.loads(node.metadata.annotations["networking.gke.io/nic-info"])
        for nic in nics:
            if nic.get("birthName") == adapter:
                return nic["birthIP"]
        logger.error(f"NIC information:\n{json.dumps(nics, indent=2)}")
    raise GracefulFallback(f"Could not find adapter {adapter} on {node.metadata.name}")


def ping_random_nodes(n_nodes=NUM_PING_TRIES):
    logger.info("Performing ping checks")
    nodes = _find_nodes_in_my_superblock(remove_self=True)

    timeout = str(float(PING_TIMEOUT_MS / 1000))
    for adapter in ADAPTERS:
        fails = 0
        subset = random.sample(nodes, k=n_nodes)  # without replacement
        for node in subset:
            nodename = node.metadata.name
            ip = _get_adapter_ip(adapter, node)
            try:
                bash(f"ping -c 1 -W 0.1 {ip}")
            except CalledProcessError:
                fails += 1
                logger.warning("Fail")
            fail_pct = fails / n_nodes
            if fail_pct > FAILURE_THRESHOLD:
                msg = f"Ping test failed on {adapter}, fail rate {fail_pct:.1%}"
                raise NeedsPowerCycle("RandomPingCheck", "RandomPingCheckFailure", msg)
    logger.info("Ping check passes")


def all_checks():
    if not is_tcpx_cluster():
        return
    try:
        ping_random_nodes()
    except GracefulFallback as gf:
        logger.warning(f"Ping test failed with an unexpected GracefulFallback: {gf}")


if __name__ == "__main__":
    logging.basicConfig(level="DEBUG")
    all_checks()
