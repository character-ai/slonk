#!/usr/bin/env python

"""
Detects maintenance events from google
"""

import logging
import urllib.error

from slonk.health.base import NeedsMaintenance
from slonk.utils import is_tcpx_cluster, curl

logger = logging.getLogger(__name__)

MAINT_URL = (
    "http://metadata.google.internal/computeMetadata/v1/instance/maintenance-event"
)
HEADERS = {"Metadata-Flavor": "Google"}


def check_for_maintenance():
    try:
        status = curl(MAINT_URL, headers=HEADERS)
    except urllib.error.HTTPError as e:
        if e.code == 404:
            logger.info(
                f"HTTP Error 404: Not Found for URL {MAINT_URL}. Skip maint check."
            )
            return
        else:
            logger.error(f"HTTP Error {e.code}: {e.reason} for URL {MAINT_URL}")
            raise
    if status != "NONE":
        msg = f"Node maintenance-event set to {status}"
        logger.error(msg)
        raise NeedsMaintenance("GoogleScheduledMaintenanceEvent", msg)


def all_checks():
    if not is_tcpx_cluster():
        return
    check_for_maintenance()


if __name__ == "__main__":
    all_checks()
