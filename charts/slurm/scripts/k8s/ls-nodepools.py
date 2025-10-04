#!/usr/bin/env python3

import os

nodepools = {}
with os.popen("kubectl get nodes --show-labels --no-headers") as f:
    for line in f:
        row = line.split()
        node = row[0]
        labels = row[-1].split(",")
        for label in labels:
            if "nodepool=" in label:
                nodepool = label.split("=")[1]
                break
        nodepools.setdefault(nodepool, []).append(node)

for nodepool, nodes in sorted(nodepools.items()):
    print(f"{nodepool}\t{len(nodes)}\t{','.join(nodes)}")
