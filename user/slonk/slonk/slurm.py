#!/usr/bin/env python3

from typing import List

import os
import logging

logger = logging.getLogger(__name__)


def get_k8s_name(slurm_name=None):
    if slurm_name is None:
        return os.env["K8S_NODE_NAME"]


def expand_names(hostlist: str):
    from slonk.utils import bash

    return bash(f"scontrol show hostnames '{hostlist}'", split_lines=True)


def collapse_names(hostnames: List[str]):
    from slonk.utils import bash

    names = " ".join(hostnames)

    return bash(f"scontrol show hostlistsorted '{names}'", split_lines=True)[0]
