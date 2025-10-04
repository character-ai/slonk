#!/bin/bash

set -e

# turn off core dumps
ulimit -c 0

echo "checking for ifconfig issues"
echo "counting adapters"
num_adapter="$(ifconfig | grep 'eth.*mtu' | wc -l)"
echo "counting ipv4"
num_inet4="$(ifconfig | grep -v inet6 | grep 'eth.*mtu' -A 1 | grep inet | wc -l)"
if [[ "$num_adapter" != "$num_inet4" ]]
then
    echo "ERROR: Adapter missing an ip address."
    ifconfig
    exit 23
fi

# Potential future enhancements:
# - check for XID errors
# - check for ECC errors
# - check dmesg for network rules
# - validate GPU count matches expected

# check nvidia smi for anything that fell off the bus
echo "running nvidia-smi"
SCRIPTS_BASE=${SLURM_SCRIPTS_PATH:-"/home/common/git-sync/k8s/k8s.git/charts/slurm/scripts"}
source ${SCRIPTS_BASE}/profile.d/h100.sh
source ${SCRIPTS_BASE}/profile.d/nvidia.sh
nvidia-smi > /dev/null

# finally prep the tempdir
export SLURM_TMPDIR=/mnt/localdisk/slurmtmp_$SLURM_JOBID
echo "making tmpdir"
mkdir -p $SLURM_TMPDIR
echo "chmod'ing tempdir"
chmod a+rwx $SLURM_TMPDIR

# kill stuff
run_kill () {
  while read -r line; do
    pid=$(echo $line | awk '{print $1}')
    etime=$(echo $line | awk '{print $2}')
    args=$(echo $line | awk '{print substr($0, index($0,$3))}')

    if [[ $pid -ne $$ ]]; then
      echo "Considering process with PID $pid, Start Time $etime, Command $args"

      mins=$(echo $etime | awk -F: '{ if (NF==2) { print $1 } else if (NF==3) { print $1*60 + $2 } else { split($1,days,"-"); print days[1]*24*60 + $2*60 + $3 }}')

      if [ $mins -gt 2 ]; then
        echo "About to kill process with PID $pid, matching pattern $1"
        kill $pid 2 || echo "Failed to kill $pid, might not exist anymore"
      else
        echo "Skipping process with PID $pid, as it is less than 2 minutes old"
      fi
    else
      echo "Skipping self, PID $$"
    fi
  done < <(ps -eo pid,etime,args | grep "$1" | grep -v grep)
}

run_kill "pretrain_gpt.py"
run_kill "train.py"
run_kill "gpu_burn"
run_kill "model_server"

