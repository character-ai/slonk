#!/bin/bash

# executed on shutdown of a compute node
scontrol update node=${K8S_POD_NAME:-$(hostname)} state=down
sleep 3
scontrol update node=${K8S_POD_NAME:-$(hostname)} state=future
