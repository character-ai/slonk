#!/usr/bin/env python3

import logging
import sys

import slonk.lifecycle.goalstate


logger = logging.getLogger(__name__)


def setup_args(args):
    lifecycle_subparsers = args.add_subparsers(dest="action", required=True)

    lifecycle_subparsers.add_parser("undrain", help="Undrain current slurm node")

    drain_parser = lifecycle_subparsers.add_parser("drain", help="Drain current slurm node")
    drain_parser.add_argument(
        "--reason", required=True, help="Reason for draining the node"
    )


def main(args):
    try:
        if args.action == "drain":
            slonk.lifecycle.goalstate.drain_slurm_node(args.reason)
        elif args.action == "undrain":
            slonk.lifecycle.goalstate.undrain_slurm_node()
    except Exception as e:
        logger.exception(e)
        sys.exit(1)
