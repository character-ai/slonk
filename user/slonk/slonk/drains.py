"""
Helper file for slonk drain analysis

Example usage:
python -m slonk drain-reasons --start_time 2023-12-24T00:51:08 --end_time 2023-12-24T07:12:08
"""
from slonk.utils import bash
from slonk.slurm import collapse_names
from datetime import datetime
import logging
import re

logger = logging.getLogger(__name__)


NODE_NAME_PREFIX = "NodeName="
REASON_PREFIX = "Reason="


def get_nodename_from_nodeline(line: str):
    node_name = line.split()[0]
    return node_name.replace(NODE_NAME_PREFIX, "")


def get_reason_from_nodeline(line: str):
    reason = line.split(REASON_PREFIX)[1].split("[")[0].strip()
    return reason


def get_time_from_nodeline(line: str):
    time = line.split()[-1]
    format_str = ""
    time = time.split("@")[1][:-1]
    return datetime.strptime(time, "%Y-%m-%dT%H:%M:%S")


def is_valid_time(time, start_time: datetime, end_time: datetime):
    is_valid = True
    if end_time and end_time < time:
        is_valid = False
    if start_time and start_time > time:
        is_valid = False
    return is_valid


def get_drain_reasons(nodes: str):
    name2reason = {}
    current_node, current_reason = None, None
    for n in nodes:
        if NODE_NAME_PREFIX in n:
            if current_node and current_reason:
                current_node = None
                current_reason = None
            current_node = get_nodename_from_nodeline(n)
            current_reason = None
        elif REASON_PREFIX in n:
            current_reason, current_time = get_reason_from_nodeline(
                n
            ), get_time_from_nodeline(n)
            name2reason[current_node] = (current_reason, current_time)

    return name2reason


def get_nodes_drains(
    start_time: datetime = None, end_time: datetime = None, reason_filter: str = ".*"
):
    nodes = bash(f"scontrol show node", split_lines=True)
    name2reason = get_drain_reasons(nodes)
    to_undrain = []
    for name in name2reason:
        if is_valid_time(name2reason[name][1], start_time, end_time):
            reason, time = name2reason[name]
            if re.search(reason_filter, reason):
                logger.info(
                    f"node name {name} was drained for reason {reason} at {time} ",
                )
                to_undrain.append(name)
    return to_undrain


def setup_args(parser):
    parser.add_argument(
        "--start",
        help="start time to filter drain nodes in format 'Y-m-dTH:M:S'",
        required=False,
    )
    parser.add_argument(
        "--end",
        help="end time to filter drain nodes in format 'Y-m-dTH:M:S'",
        required=False,
    )
    parser.add_argument(
        "--reason",
        default=".*",
        help="Filter to reasons matching a given regex",
    )
    parser.add_argument(
        "--undrain", action="store_true", help="If given, also undrain the nodes"
    )


def main(args):
    start_time, end_time = args.start, args.end
    if start_time:
        start_time = datetime.strptime(start_time, "%Y-%m-%dT%H:%M:%S")
    if end_time:
        end_time = datetime.strptime(end_time, "%Y-%m-%dT%H:%M:%S")
    nodes = get_nodes_drains(start_time, end_time, reason_filter=args.reason)
    if args.undrain:
        confirm = input(f"Do you want to undrain {len(nodes)} nodes? y to continue: ")
        if confirm.strip() == "y":
            bash(
                f"scontrol update node={collapse_names(nodes)} state=resume reason=",
                sudo=True,
            )
