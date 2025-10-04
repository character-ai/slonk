#!/usr/bin/env python3

"""
A prometheus-friendly flask app for monitoring jobs from grafana
"""

import argparse
import textwrap
import os
import subprocess
import flask
from flask import request
import socket
import re
import threading

app = flask.Flask(__name__)

LOCK = threading.Lock()
LAST_METRICS = {}


@app.route("/health")
def health():
    return "OK"


@app.route("/metrics")
def metrics():
    global LAST_METRICS, LOCK

    with LOCK:
        to_write = LAST_METRICS.copy()
        LAST_METRICS.clear()
    retval = ""
    for metric, value in to_write.items():
        retval += f"{metric} {value}\n"
    return retval


def _clean_name(name):
    return re.sub(r"\W", "", name.replace("/", "_"))


@app.route("/write", methods=["POST"])
def write():
    global LAST_METRICS, LOCK

    data = request.json
    towrite = {}
    for series in data:
        metric_name = _clean_name(series.pop("__name__"))
        metric_value = series.pop("__value__")
        labels = []
        for key in sorted(series.keys()):
            value = series[key]
            labels.append(f'{_clean_name(key)}="{value}"')
        labels_str = ",".join(labels)
        metric_str = f"jobmetrics_{metric_name}{{{labels_str}}}"
        towrite[metric_str] = metric_value
    with LOCK:
        for metric_str, metric_value in towrite.items():
            LAST_METRICS[metric_str] = metric_value
    return "OK"


def main():
    parser = argparse.ArgumentParser("job_metrics")
    args = parser.parse_args()
    app.run(host="0.0.0.0", port=7080, debug=False)


if __name__ == "__main__":
    main()
