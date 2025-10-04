#!/bin/bash

if [ -e "/usr/local/cuda-11" ]; then
    # provided by the nvcr.io/nvidia/pytorch images
    export LD_LIBRARY_PATH="$CUDA_HOME/lib64:/usr/local/nvidia/lib64:/usr/local/cuda/compat $LD_LIBRARY_PATH"
    export LD_LIBRARY_PATH="/usr/local/lib/python3.8/dist-packages/torch/lib:/usr/local/lib/python3.8/dist-packages/torch_tensorrt/lib:/usr/local/cuda/compat/lib:/usr/local/nvidia/lib:/usr/local/nvidia/lib64:/usr/local/cuda-11/lib64:${LD_LIBRARY_PATH}"
    export LIBRARY_PATH="/usr/local/cuda/lib64/stubs:${LIBRARY_PATH}"
    export PATH="/usr/local/nvm/versions/node/v16.15.1/bin:/usr/local/lib/python3.8/dist-packages/torch_tensorrt/bin:/usr/local/mpi/bin:/usr/local/nvidia/bin:/usr/local/cuda/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/local/ucx/bin:/opt/tensorrt/bin:${PATH}"
    export _CUDA_COMPAT_PATH="/usr/local/cuda/compat"
fi
if [ -e "/usr/local/cuda-12.2" ]; then
    export CUDA_HOME=/usr/local/cuda
    export LD_LIBRARY_PATH="$CUDA_HOME/lib64:$CUDA_HOME/compat:$CUDA_HOME/lib64/stubs:$LD_LIBRARY_PATH"
    export PATH="$CUDA_HOME/bin:$CUDA_HOME/bin:$PATH"
    export _CUDA_COMPAT_PATH="/usr/local/cuda/compat"
    export TRITON_PTXAS_PATH=$(which ptxas)
    export ENABLE_TMA=1
    export ENABLE_MMA_V3=1
fi
if [ -e "/usr/local/cuda-12.3" ]; then
    export CUDA_HOME=/usr/local/cuda
    export LD_LIBRARY_PATH="$CUDA_HOME/lib64:$CUDA_HOME/compat:$CUDA_HOME/lib64/stubs:$LD_LIBRARY_PATH"
    export PATH="$CUDA_HOME/bin:$CUDA_HOME/bin:$PATH"
    export _CUDA_COMPAT_PATH="/usr/local/cuda/compat"
    export TRITON_PTXAS_PATH=$(which ptxas)
    export ENABLE_TMA=1
    export ENABLE_MMA_V3=1
fi
if [ -e "/usr/local/cuda-12.4" ]; then
    export CUDA_HOME=/usr/local/cuda
    export LD_LIBRARY_PATH="$CUDA_HOME/lib64:$CUDA_HOME/compat:$CUDA_HOME/lib64/stubs:$LD_LIBRARY_PATH"
    export PATH="$CUDA_HOME/bin:$CUDA_HOME/bin:$PATH"
    export _CUDA_COMPAT_PATH="/usr/local/cuda/compat"
    export TRITON_PTXAS_PATH=$(which ptxas)
    export ENABLE_TMA=1
    export ENABLE_MMA_V3=1
fi

