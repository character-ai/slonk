#!/usr/bin/env python3

import logging
import sys
import time


from slonk.health.base import HealthCheckError
from slonk.utils import is_nvidia_cluster, gethostname
import slonk.k8s as k8s

import slonk.health.nvidia_smi
import slonk.health.xid
import slonk.health.iface
import slonk.health.htod
import slonk.health.weka
import slonk.health.lspci
import slonk.health.nccl
import slonk.health.link_flap
import slonk.health.ibv_devinfo
import slonk.health.gpu_burn
import slonk.health.dcgmi
import slonk.health.serial
import slonk.health.cpu_load
import slonk.health.ping
import slonk.health.disk
import slonk.health.controller
import slonk.health.maint
import slonk.health.k8s

logger = logging.getLogger(__name__)

SLEEP_TIME_SEC = 60.0


def setup_args(args):
    args.add_argument(
        "--no-mitigation",
        action="store_true",
        help="Avoid taking any actions after tests",
    )
    args.add_argument(
        "--mode",
        choices={"fast", "all", "noninvasive", "continuous"},
        default="all",
        help="Run mode",
    )


def run_lifecycle_checks(args):
    slonk.health.k8s.all_checks()


def run_noninvasive_checks(args):
    slonk.health.serial.all_checks()
    slonk.health.iface.all_checks()
    slonk.health.lspci.all_checks()
    slonk.health.weka.all_checks()
    slonk.health.controller.all_checks()

    # intentionally not running the maint test, since noninvasives
    # run in the background

    if is_nvidia_cluster():
        slonk.health.ibv_devinfo.all_checks()
        slonk.health.nvidia_smi.all_checks()
        slonk.health.xid.all_checks()


def run_fast_checks(args, skip_redundant=False):
    run_noninvasive_checks(args)

    # not a continuous check since disk could vary over time of jobs
    slonk.health.disk.all_checks()
    slonk.health.cpu_load.all_checks()
    slonk.health.maint.all_checks()

    if is_nvidia_cluster():
        if not skip_redundant:
            slonk.health.dcgmi.all_checks(fast=True)
            # even fast nvlink checks are a tad slow. we run them on pod start tho
            # slonk.health.nccl.fast_nvlink_check()


def run_health_checks(args):
    run_fast_checks(args, skip_redundant=True)
    slonk.health.ping.all_checks()
    if is_nvidia_cluster():
        slonk.health.gpu_burn.all_checks()
        slonk.health.dcgmi.all_checks(fast=False)
        slonk.health.htod.all_checks()
        slonk.health.nccl.all_checks()


def run_continuous_checks(args):
    while True:
        run_noninvasive_checks(args)
        logger.debug(f"Sleeping {SLEEP_TIME_SEC}s...")
        time.sleep(SLEEP_TIME_SEC)


def main(args):
    logger.log(
        logging.INFO,
        f"Beginning {args.mode} health checks on {gethostname()}",
    )
    try:
        if args.mode == "noninvasive":
            run_noninvasive_checks(args)
        elif args.mode == "fast":
            run_fast_checks(args)
        elif args.mode == "continuous":
            run_continuous_checks(args)
        elif args.mode == "all":
            run_health_checks(args)
        logger.log(logging.INFO, "All health checks pass")
    except HealthCheckError as hce:
        # TODO: implement remediation
        logger.critical(f"Failed health checks: {hce} ({hce.__class__.__name__})")
        if args.no_mitigation:
            logger.warning("Mitigations skipped due to --no-mitigations.")
            sys.exit(1)
        elif hce.handle_mitigation():
            sys.exit(0)
        else:
            logger.warning("Mitigation invoked and returned false. Exiting.")
            sys.exit(1)
    except Exception:
        logger.exception("Unknown exception")
        sys.exit(240)
