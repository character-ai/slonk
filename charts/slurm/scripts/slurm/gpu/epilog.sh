#!/bin/bash

export SLURM_TMPDIR=/mnt/localdisk/slurmtmp_$SLURM_JOBID
rm -rf $SLURM_TMPDIR || true

# Optionally reboot after job completion (configure via ENABLE_GPU_REBOOT=true)
if [ "${ENABLE_GPU_REBOOT:-false}" = "true" ]; then
    scontrol reboot ASAP nextstate=RESUME $HOSTNAME
fi
