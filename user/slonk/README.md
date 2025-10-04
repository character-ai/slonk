# Slonk Python Package

Python package providing health monitoring, lifecycle management, and utilities for Slonk SLURM clusters.

## Installation

```bash
pip install -e .
# or
python setup.py install
```

## Components

### Health Checks (`slonk/health/`)

Automated health monitoring system for compute nodes:

- **GPU checks**: `nvidia_smi.py`, `dcgmi.py`, `gpu_burn.py`, `xid.py`
- **Network**: `ping.py`, `link_flap.py`, `nccl.py`
- **Storage**: `disk.py`, `weka.py`
- **System**: `cpu_load.py`, `serial.py`, `lspci.py`
- **Controller**: `controller.py` - orchestrates all health checks

**Usage**:
```python
from slonk.health.controller import run_health_checks
run_health_checks()
```

### Lifecycle Management (`slonk/lifecycle/`)

Node lifecycle and goal state management:
- `goalstate.py`: Manages node state transitions (draining, rebooting, resuming)

### Utilities

- `slurm.py`: SLURM command wrappers and helpers
- `k8s.py`: Kubernetes API interactions
- `env.py`: Environment variable setup for SLURM nodes
- `drains.py`: Node draining logic
- `reboot.py`: Safe node reboot procedures
- `fingerprint.py`: Node hardware fingerprinting

## Kubernetes Operator (`operators/slonklet/`)

Go-based Kubernetes operator managing custom resources:

**CRDs**:
- `PhysicalNode`: Represents physical/VM nodes in SLURM cluster
- `SlurmJob`: Kubernetes representation of SLURM jobs

**Development**:
```bash
cd operators/slonklet
make manifests   # Generate CRDs and RBAC
make install     # Install CRDs to cluster
make run         # Run operator locally
make docker-build IMG=your-registry/slonklet:tag
```

## Adding Custom Health Checks

1. Create new file in `slonk/health/your_check.py`:
```python
from slonk.health.base import HealthCheck

class YourHealthCheck(HealthCheck):
    def check(self):
        # Check logic here
        # Raise exception on failure
        pass
```

2. Register in `slonk/health/__init__.py`

3. Add to controller's check list if needed

## Testing

```bash
pytest tests/
```
