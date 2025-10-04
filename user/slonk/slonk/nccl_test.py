#!/usr/bin/env python3

from typing import List

import logging
import math
import subprocess
import multiprocessing
import os
import sys
import random

from multiprocessing.pool import ThreadPool


from slonk.health.base import NeedsManual
from slonk.health.nccl import get_nccl_envvars
from slonk.slurm import expand_names, collapse_names
from slonk.utils import bash

logger = logging.getLogger(__name__)

TIMEOUT = 60
# Runs a sweep between MSG_SIZE_START and MSG_SIZE_END.
MSG_SIZE_MB = int(os.environ.get("NCCL_TEST_SIZE_MB", 2 * 1024))
VALID_OPERATIONS = (
    "all_gather",
    "all_reduce",
    "alltoall",
    "broadcast",
    "gather",
    "hypercube",
    "reduce",
    "reduce_scatter",
    "scatter",
    "sendrecv",
)
OPERATION = os.environ.get("NCCL_TEST_OP", "all_reduce")
MPI_FLAGS = "--map-by ppr:8:node -bind-to numa --mca plm_rsh_agent ssh -mca btl_tcp_if_include eth0"

assert OPERATION in VALID_OPERATIONS


def setup_args(parser):
    parser.add_argument("nodes", help="List of nodes")


def run_nccl(nodelist: List[str], dry_run: bool = False):
    logger.info(
        f"Running nccl test on {collapse_names(nodelist)} ({len(nodelist)} nodes)"
    )
    envvars = get_nccl_envvars()
    envvars["NCCL_DEBUG"] = "INFO"
    nccl_flags = " ".join(f"-x {flag}='{val}'" for flag, val in envvars.items())
    mainnode = nodelist[0]
    hosts = ",".join(f"{h}:8" for h in nodelist)
    np = len(nodelist) * 8
    mpi_flags = f"{MPI_FLAGS} -H {hosts} -np {np}"
    MSG_SIZE = MSG_SIZE_MB * 2**20
    cmd = (
        f"ssh {mainnode} "
        f'"mpirun {mpi_flags} {nccl_flags} '
        f'{OPERATION}_perf -g 1 -f 2 -b {MSG_SIZE} -e {MSG_SIZE}"'
    )
    try:
        logger.debug(f"Running command `{cmd}`")
        if dry_run:
            return -255
        output = bash(cmd, silent_stderr=True, timeout=TIMEOUT)
    except subprocess.TimeoutExpired as te:
        logger.error(
            f"Command {cmd} timed out:\nstdout:\n{te.output}\nstderr:\n{te.stderr}"
        )
        logger.error(
            f"You may still have processes lingering on nodes. "
            f"Run `pdsh -w '{nodelist}' pkill all_reduce_perf`"
        )
        return 0.0
    except subprocess.CalledProcessError as cpe:
        logger.error(
            f"Command {cmd} failed:\nstdout:\n{cpe.output}\nstderr:\n{cpe.stderr}"
        )
        return 0.0
    except KeyboardInterrupt as ki:
        logger.error(
            f"You may still have processes lingering on nodes. "
            f"Run `pdsh -w '{nodelist}' pkill all_reduce_perf`"
        )
    logger.debug("NCCL test output:\n" + output.rstrip())
    for line in output.strip().split("\n"):
        line = line.strip()
        if line.startswith("# "):
            # comments from mpirun, ignore
            continue
        if str(MSG_SIZE) in line:
            # ex:
            # 2147483648     536870912     float     sum      -1    12590  170.57  319.83      0    12555  171.05  320.71      0
            msg_size, *_, busbw, errors = line.split()
            assert msg_size == str(MSG_SIZE)
            busbw = float(busbw)
            errors = int(errors)
            if errors > 0:
                msg = f"Non-zero errors ({errors}) on nccl test of {nodelist}"
                logger.error(msg)
                raise NeedsManual("NCCLTest", "NCCLTestFailure", msg + "\n" + output)
            return busbw


def nccl_bisect(nodelist: List[str], seed=None, depth=1):
    logger.info(f"Running bisect on {len(nodelist)} nodes: {collapse_names(nodelist)}")
    # shuffle for good measure
    nodelist = sorted(nodelist)
    random.Random(seed).shuffle(nodelist)

    if len(nodelist) <= 3:
        logger.info(
            f"Down to {len(nodelist)} nodes, switching to pairwise: {collapse_names(nodelist)}"
        )
        return nccl_pairwise(nodelist)

    a_list, b_list = nodelist[::2], nodelist[1::2]

    a_measure = run_nccl(a_list)
    logger.info(f"Measurment of {collapse_names(a_list)}: {a_measure:.1f}")
    b_measure = run_nccl(b_list)
    logger.info(f"Measurment of {collapse_names(b_list)}: {b_measure:.1f}")

    delta = a_measure - b_measure
    delta_pct = delta / (1e-5 + max(a_measure, b_measure))

    if delta_pct >= 0.05:
        # b is slower
        return nccl_bisect(b_list, depth=depth + 1)
    elif delta_pct <= -0.05:
        # a is slower
        return nccl_bisect(a_list, depth=depth + 1)
    else:
        logger.info("Performance was similar. Bisecting both halves.")
        return nccl_bisect(a_list, depth=depth + 1) + nccl_bisect(
            b_list, depth=depth + 1
        )


def _split_parallel(pairs_to_run):
    running_now = []
    not_running_yet = []
    spoken_for = set()
    for a, b in pairs_to_run:
        if a in spoken_for or b in spoken_for:
            not_running_yet.append((a, b))
        else:
            running_now.append((a, b))
            spoken_for.update([a, b])
    return running_now, not_running_yet


def _run_parallel(pairs):
    results = {}
    with ThreadPool(processes=len(pairs)) as pool:
        scores = pool.map(run_nccl, pairs)
        for s, (l, r) in zip(scores, pairs):
            assert l not in results
            assert r not in results
            results[l] = s
            results[r] = s
    return results


def nccl_pairwise(nodelist: List[str], seed=None):
    logger.info(
        f"Running pairwise nccl test on {len(nodelist)} nodes: {collapse_names(nodelist)}"
    )

    nodelist = sorted(nodelist)
    random.Random(seed).shuffle(nodelist)

    processes = []
    aggregated = {}

    # emulate the cycle by appending the first to the end
    nodelist_cycle = nodelist.copy()
    nodelist_cycle.append(nodelist[0])
    pairs = []
    for pair in zip(nodelist_cycle, nodelist_cycle[1:]):
        pairs.append(pair)

    aggregated = {}
    times_seen = {}
    pairs_to_run = pairs.copy()
    while pairs_to_run:
        running_now, pairs_to_run = _split_parallel(pairs_to_run)
        results = _run_parallel(running_now)
        for node, score in results.items():
            aggregated[node] = max(aggregated.get(node, 0), score)
            times_seen[node] = times_seen.get(node, 0) + 1

    # make sure we've tested everyone
    assert len(aggregated) == len(nodelist)
    assert len(times_seen) == len(nodelist)
    for node, seen in times_seen.items():
        assert seen == 2, f"{node} not paired enough times"

    mean = sum(aggregated.values()) / len(aggregated)
    logger.info(f"Avg perf: {mean:.1f}")

    sorted_ = sorted(list(aggregated.items()), key=lambda x: x[1])
    n = math.ceil(len(aggregated) / 2)
    logger.info(f"Scores: {sorted_}")
    median = sorted_[:n][-1][1]
    sus = []
    for k, v in sorted_:
        if v == 0.0 or (v - median) / (median + 1e-5) < -0.1:
            sus.append(k)
    logger.info(f"Sus nodes: {collapse_names(sus)}")
    return sus


def main_bisect(args):
    retval = nccl_bisect(expand_names(args.nodes))
    logger.log(
        logging.INFO, f"Bisect returned {collapse_names(retval)} as suspicious"
    )


def main_pairwise(args):
    retval = nccl_pairwise(expand_names(args.nodes))
    logger.log(
        logging.INFO, f"Bisect returned {collapse_names(retval)} as suspicious"
    )


def main(args):
    logger.info(f"Beginning NCCL tests on {args.nodes}")
    gbps = run_nccl(expand_names(args.nodes))
    logger.log(logging.INFO, f"NCCL performance: {gbps}")
