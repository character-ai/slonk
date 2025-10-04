#!/bin/bash

set -e

function _join() {
    local IFS=$1
    shift
    echo "$*"
}

EXTRA_INSTALL_LIST=(
    libnccl2=2.20.5-1+cuda12.4
    libnccl-dev=2.20.5-1+cuda12.4
    libcudnn8
    libcudnn8-dev
    datacenter-gpu-manager
    ibverbs-utils
    infiniband-diags
    libsox-dev
    ffmpeg
    ipmitool
    mft
    pciutils
    ibutils2
)

export BASE_IMAGE=nvidia/cuda:12.4.1-devel-ubuntu22.04
export EXTRA_INSTALL=$(_join " " ${EXTRA_INSTALL_LIST[@]})
export EXTRA_PIP_INSTALL="gpustat"
export EXTRA_SH="extra.sh"
export LD_LIBRARY_PATH=/usr/local/cuda/lib64:/usr/local/cuda/compat:/usr/local/cuda/lib64/stubs:/usr/local/nvidia/lib64
cp extra.sh ../slonk/
cd ../slonk && \
    docker build \
        --build-arg BASE_IMAGE \
        --build-arg EXTRA_INSTALL \
        --build-arg EXTRA_PIP_INSTALL \
        --build-arg LD_LIBRARY_PATH \
        "$@" . && \
    cd -
rm ../slonk/extra.sh
