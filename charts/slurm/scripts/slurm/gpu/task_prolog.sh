#!/bin/bash
set -e

# turn off core dumps
ulimit -c 0

cat /etc/profile.d/k8s_env.sh | grep export | sed 's/"//g'
echo "export PROLOG_NODE=$(hostname)"
echo "export RCLONE_FAST_LIST=1"
echo "export LC_TIME=C.UTF-8"
export SLURM_TMPDIR=/mnt/localdisk/slurmtmp_$SLURM_JOBID
echo "export SLURM_TMPDIR=$SLURM_TMPDIR"
