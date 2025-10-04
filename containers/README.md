# Container Images

This directory contains Dockerfile definitions and build scripts for Slonk container images.

## Images

### slonk (Base Image)

General-purpose SLURM node image for login, controller, and standard compute nodes.

**Build**:
```bash
cd slonk
./build.sh -t your-registry/slonk:latest
```

**Includes**: SLURM, Python environment, LDAP tools, health check scripts

### slonk-h100 (GPU-Optimized)

Specialized image for H100 GPU nodes with CUDA 12.4, NCCL 2.20, and GPU monitoring tools.

**Build**:
```bash
cd slonk-h100
./build.sh -t your-registry/slonk-h100:latest
```

**Additional packages**: NVIDIA drivers, DCGM, InfiniBand tools, cuDNN

## Customization

- **Base image**: Modify `BASE_IMAGE` in build scripts
- **Extra packages**: Add to `EXTRA_INSTALL_LIST` in `slonk-h100/build.sh`
- **Python packages**: Set `EXTRA_PIP_INSTALL` environment variable

## Container Structure

All containers include:
- SLURM daemon binaries (slurmctld, slurmd, slurmdbd)
- Health check Python package (`slonk`)
- Configuration management scripts
- Monitoring and logging tools
