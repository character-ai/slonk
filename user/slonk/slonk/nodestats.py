#!/usr/bin/env python3


import sys
import logging
from collections import defaultdict

from slonk.utils import bash, get_cluster
from slonk.slurm import collapse_names

logger = logging.getLogger(__name__)


def setup_args(parser):
    pass


def main(args=None):
    states = bash(
        'sinfo -h --partition general -N --partition priority -o "%N\t%f\t%t"',
        split_lines=True,
    )
    results = defaultdict(set)
    all_nf = set()
    for line in states:
        node, node_features_, state = line.split("\t")
        state = (
            state.replace("mix", "alloc")
            .replace("drng@", "alloc")
            .replace("alloc@", "alloc")
            .replace("maint", "resv")
            .replace("plnd", "down")
        )
        if "*" in state:
            state = "down"
        node_features = node_features_.split(",")
        for nf in node_features:
            if not nf.startswith(get_cluster()):
                continue
            results[(nf, state)].add(node)
            results[(nf, "total")].add(node)
            all_nf.add(nf)

    keys = sorted(list(results.keys()))
    last_feature = None
    for key in keys:
        feature, state = key
        if sys.stdout.isatty() and last_feature and last_feature != feature:
            logger.info("")
        nodes_ = results[key]
        nodes = collapse_names(list(nodes_))
        pstr = f"{feature:20s} {state:8s} {len(nodes_): 3d}  {nodes}"

        if not sys.stdout.isatty():
            if state != "total":
                print(pstr)
        elif state == "alloc":
            logger.log(logging.INFO, pstr)
        elif state == "idle" or state == "resv":
            logger.info(pstr)
        elif state == "total":
            logger.log(logging.INFO, pstr)
        elif state == "down":
            logger.error(pstr)
        else:
            logger.warning(pstr)

        last_feature = feature


if __name__ == "__main__":
    
    main()
