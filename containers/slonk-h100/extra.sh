#!/bin/bash
set -e
set -x

export INSTALL_DIR=/usr/local/bin

cd /build
git clone https://github.com/NVIDIA/nccl-tests.git
cd nccl-tests
make MPI=1 \
    MPI_HOME=/usr/lib/x86_64-linux-gnu/openmpi/ \
    NVCC_GENCODE="-gencode=arch=compute_80,code=sm_80 -gencode=arch=compute_86,code=sm_86 -gencode=arch=compute_90,code=sm_90"
cp build/{all_gather_perf,all_reduce_perf,alltoall_perf,broadcast_perf,gather_perf,hypercube_perf,reduce_perf,reduce_scatter_perf,scatter_perf,sendrecv_perf} $INSTALL_DIR/
cd /build
rm -rf nccl-tests

cd /build
git clone https://github.com/wilicc/gpu-burn.git
cd gpu-burn
sed gpu_burn-drv.cpp -i -e "s^#define COMPARE_KERNEL \"compare.ptx\"^#define COMPARE_KERNEL \"$INSTALL_DIR/gpu_burn_compare.ptx\"^"
make
cp compare.ptx $INSTALL_DIR/gpu_burn_compare.ptx
cp gpu_burn $INSTALL_DIR/gpu_burn
cd /build
rm -rf gpu-burn

cd /build
git clone https://github.com/NVIDIA/cuda-samples.git
cd cuda-samples
rm -rf Samples/{0,2,3,4,5,6,7}*
make
cp Samples/1_Utilities/bandwidthTest/bandwidthTest $INSTALL_DIR/nv-bandwidthtest
cd /build
rm -rf cuda-samples

# NSYS Profiler
cd /build
wget -c https://developer.nvidia.com/downloads/assets/tools/secure/nsight-systems/2023_4_1_97/nsightsystems-linux-cli-public-2023.4.1.97-3355750.deb
dpkg -i nsightsystems-linux-cli-public-2023.4.1.97-3355750.deb
rm nsightsystems-linux-cli-public-2023.4.1.97-3355750.deb
