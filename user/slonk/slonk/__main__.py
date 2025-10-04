#!/usr/bin/env python3

import sys
import argparse
import logging

from slonk import health
from slonk import prolog
from slonk import nccl_test
from slonk import drains
from slonk import reboot
from slonk import fingerprint
from slonk import env
from slonk import slonkme
from slonk import nodestats
from slonk import lifecycle


def main():
    main_parser = argparse.ArgumentParser("slonk")
    main_parser.add_argument("--verbose", action="store_true")
    main_parser.add_argument("--silent", action="store_true")
    main_parser.add_argument("--quiet", action="store_true")
    main_parser.set_defaults(func=lambda a: main_parser.print_help())
    subparsers = main_parser.add_subparsers()

    # prologs and epilogs
    parser = subparsers.add_parser("prolog", help="Run the prolog")
    prolog.setup_args(parser)
    parser.set_defaults(func=prolog.main)

    # on node health checks
    parser = subparsers.add_parser("health", help="Run health checks")
    health.setup_args(parser)
    parser.set_defaults(func=health.main)

    # multinode nccl tests
    parser = subparsers.add_parser("nccl-test", help="Run multi-node nccl tests")
    nccl_test.setup_args(parser)
    parser.set_defaults(func=nccl_test.main)

    parser = subparsers.add_parser("nccl-bisect", help="Run multi-node nccl bisect")
    nccl_test.setup_args(parser)
    parser.set_defaults(func=nccl_test.main_bisect)

    parser = subparsers.add_parser(
        "nccl-pairwise", help="Run multi-node nccl pairwise search"
    )
    nccl_test.setup_args(parser)
    parser.set_defaults(func=nccl_test.main_pairwise)

    parser = subparsers.add_parser(
        "drain-reasons", help="filter for drains reasons in a time frame"
    )
    drains.setup_args(parser)
    parser.set_defaults(func=drains.main)

    parser = subparsers.add_parser("fingerprint", help="prints this node's uuid")
    fingerprint.setup_args(parser)
    parser.set_defaults(func=fingerprint.main)

    parser = subparsers.add_parser("lifecycle", help="manage node's lifecycle")
    lifecycle.setup_args(parser)
    parser.set_defaults(func=lifecycle.main)

    parser = subparsers.add_parser("reboot", help="power cycles the current node")
    reboot.setup_args(parser)
    parser.set_defaults(func=reboot.main)

    parser = subparsers.add_parser("env", help="prints the env vars we always want")
    env.setup_args(parser)
    parser.set_defaults(func=env.main)

    parser = subparsers.add_parser("me", help="Get Slonked!")
    slonkme.setup_args(parser)
    parser.set_defaults(func=slonkme.main)

    parser = subparsers.add_parser("nodestats", help="Print node summary stats")
    nodestats.setup_args(parser)
    parser.set_defaults(func=nodestats.main)

    args = main_parser.parse_args()

    # Setup logging
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )

    root_logger = logging.getLogger()
    for handler in list(root_logger.handlers):
        if isinstance(handler, logging.StreamHandler) and handler.stream == sys.stdout:
            if args.silent:
                root_logger.removeHandler(handler)
            elif args.quiet:
                handler.setLevel(logging.WARNING)
            elif args.verbose:
                handler.setLevel(logging.DEBUG)

    return args.func(args)


if __name__ == "__main__":
    main()
