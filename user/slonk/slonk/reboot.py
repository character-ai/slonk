#!/usr/bin/env python3

from slonk.health.base import NeedsPowerCycle


def setup_args(parser):
    pass


def main(args):
    NeedsPowerCycle(
        "ManualReboot", "ManualRebootRequested", "manual power cycle"
    ).handle_mitigation()
