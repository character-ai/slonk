#!/usr/bin/env python3

"""
A prometheus-friendly flask app for monitoring jobs from grafana
"""

import argparse
import textwrap
import os
import subprocess
import flask
import pandas as pd
import socket
import re
from functools import lru_cache

DEFAULT_PARTITION = "general"

app = flask.Flask(__name__)


def bash(cmd, silent_stderr=False):
    stderr = subprocess.DEVNULL if silent_stderr else None
    return subprocess.check_output(cmd, shell=True, stderr=stderr).decode()


def sinfo():
    raw = bash('sinfo -N --format "%N %D %P %t %G" --partition general')
    items = []
    for row in raw.strip().split("\n"):
        if "PARTITION" in row:
            # header
            continue
        node, count, partition, state, gres = row.split()
        if gres == "(null)":
            gres, gres_count = "cpu", 1
        else:
            gres, gres_count = gres.split(":")
            gres_count = int(gres_count)
        partition = partition.replace("*", "")  # default partition
        count = int(count)
        if partition != DEFAULT_PARTITION:
            continue
        if "~" in state:
            continue
        if "*" in state:
            state = "down"
        state = state.replace("mix", "alloc")

        item = {
            "node": node,
            "partition": partition,
            "state": state,
            "nnodes": count,
            "gres": gres,
            "gres_count": gres_count * count,
        }
        items.append(item)
    return pd.DataFrame(items)


def get_node_gres(nodename):
    for line in bash(f"scontrol show node '{nodename}'").split("\n"):
        line = line.strip()
        if line.startswith("Gres="):
            line = line.replace("Gres=", "")
            gres, count = line.split(":")
            count = int(count)
            return gres, count


def squeue():
    fields = "%i %u %T %D %b %P %R %j".replace(" ", "\t")
    raw = bash(f'squeue --all -r -h -o "{fields}"')
    items = []
    for line in raw.split("\n"):
        line = line.strip()
        if not line:
            continue
        (
            jobid,
            user,
            state,
            nnodes,
            gres,
            partition,
            nodelist,
            jobname,
        ) = line.split("\t")
        nnodes = int(nnodes)
        if "held" in nodelist or "Hold" in nodelist or "Held" in nodelist:
            state = "Hold"
        if "Dependency" in nodelist:
            state = "Dep"
        if "AssocGrp" in nodelist:
            state = "Quota"
        if "Priority" in nodelist:
            state = "Pending"
        if jobname in {"wrap", "zsh", "bash"}:
            state = "dev"

        # hacks for finding out the resources needed
        if gres == "N/A":
            try:
                gres, gres_count = get_node_gres(nodelist)
            except:
                gres, gres_count = "", 0
        else:
            try:
                gres, gres_count = gres.split(":")
            except ValueError:
                _, gres, gres_count = gres.split(":")
            gres_count = int(gres_count)

        state = state.title()

        partition = partition.split(",")[0]  # assume first partition for pending
        nodes = bash(f"scontrol show hostnames '{nodelist}'").strip().split("\n")
        for i, node in enumerate(nodes):
            item = {
                "jobid": jobid,
                "node": node,
                "user": user,
                "state": state,
                "nnodes": 1 if i == 0 else 0,
                "gres": gres,
                "gres_count": gres_count if i == 0 else 0,
                "jobs": 1 if i == 0 else 0,
                "partition": partition,
            }
            items.append(item)
    df = pd.DataFrame(items)
    return df


def generate():
    retval = ""

    df_sinfo = sinfo()
    df_squeue = squeue()

    # uniqify per node
    if not df_sinfo.empty:
        by_node = df_sinfo.groupby("node").head(1)
        by_state = by_node.groupby(["state", "gres"]).agg(
            {"nnodes": "sum", "gres_count": "sum"}
        )

        for (state, gres), values in by_state.iterrows():
            retval += (
                f'sinfo_gres{{state="{state}",gres="{gres}"}} {values["gres_count"]}\n'
            )
            retval += (
                f'sinfo_nodes{{state="{state}",gres="{gres}"}} {values["nnodes"]}\n'
            )

    if not df_squeue.empty:
        by_user = df_squeue.groupby(["state", "user", "gres"]).agg(
            {"gres_count": "sum", "nnodes": "sum", "jobs": "sum"}
        )
        for (state, user, gres), values in by_user.iterrows():
            retval += f'squeue_gres{{user="{user}",state="{state}",gres="{gres}"}} {values["gres_count"]}\n'
            retval += f'squeue_nodes{{user="{user}",state="{state}",gres="{gres}"}} {values["nnodes"]}\n'

    return retval


@app.route("/health")
def health():
    bash("sinfo")
    return "OK"


@app.route("/metrics")
def metrics():
    try:
        return generate()
    except subprocess.CalledProcessError:
        return ""


def main():
    parser = argparse.ArgumentParser("slurm_metrics")
    parser.add_argument("--single", action="store_true", help="Print metrics and exit.")
    args = parser.parse_args()
    if args.single:
        print(generate().rstrip())
    else:
        app.run(host="0.0.0.0", port=8071, debug=False)


if __name__ == "__main__":
    main()
