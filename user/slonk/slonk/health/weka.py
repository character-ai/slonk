#!/usr/bin/env python3

"""
Health checks related to storage systems
"""

import logging
import os

from slonk.utils import bash, get_cluster
from slonk.health.base import NeedsManual, CalledProcessError


logger = logging.getLogger(__name__)


def check_home_mount():
    if not os.path.exists("/home/common/git-sync"):
        msg = "Missing /home"
        logger.error(msg)
        raise NeedsManual("WekaMountCheck", "WekaMountCheckMissing", msg)


def check_pings():
    try:
        # Ping storage cluster IPs if configured
        ceph_ips = os.environ.get("CEPH_CLUSTER_IPS", "").split(",")
        for ip in ceph_ips:
            if ip.strip():
                bash(f"timeout 1.0 ping -c 1 {ip.strip()}")

        # Ping Weka cluster if configured
        weka_ip = os.environ.get("WEKA_CLUSTER_IP")
        if weka_ip:
            bash(f"timeout 1.0 ping -c 1 {weka_ip}")

    except CalledProcessError:
        msg = "Could not ping storage cluster"
        logger.error(msg)
        raise NeedsManual("WekaPingCheck", "WekaPingCheckError", msg)


def check_blob():
    # Check blob storage if configured
    blob_endpoint = os.environ.get("BLOB_STORAGE_ENDPOINT")
    if not blob_endpoint:
        logger.info("No blob storage endpoint configured, skipping check")
        return

    try:
        bash(f"timeout 1.0 curl -I 'http://{blob_endpoint}:80/'")
    except CalledProcessError as cpe:
        msg = "Could not reach blob storage"
        logger.error(msg)
        raise NeedsManual("WekaBlobCheck", "WekaBlobCheckError", msg)


def all_checks():
    if get_cluster() != "cluster1":
        logger.info("Skipping storage tests")
        return

    check_home_mount()
    logger.info("weka tests pass")
