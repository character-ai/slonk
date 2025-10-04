#!/bin/sh

source /home/common/git-sync/k8s/k8s.git/charts/slurm/scripts/profile.d/utils.sh

rsync -av --delete /home/common/conda-envs /mnt/localdisk/

if [ -z "$(ls /mnt/localdisk/conda-envs/)" ]; then
    exit 0
fi

for i in /mnt/localdisk/conda-envs/*
do
    activate-squashfs $i
done

