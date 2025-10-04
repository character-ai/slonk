# Slonk - Kubernetes SLURM Cluster Manager

A production-ready Kubernetes-native SLURM cluster solution with advanced GPU management, automated health monitoring, and custom operator support.

## Features

- **Full SLURM on Kubernetes**: Complete SLURM deployment with controller, login, and compute nodes
- **GPU/TPU Support**: First-class support for NVIDIA GPUs (including H100) and Google Cloud TPUs
- **Health Monitoring**: Automated node health checks with remediation capabilities
- **Custom Operator**: Kubernetes operator (`slonklet`) for managing physical nodes and SLURM jobs via CRDs
- **Multi-cloud Ready**: Designed for cloud-agnostic deployment with GCP-specific optimizations
- **LDAP Integration**: User authentication and synchronization with 2FA support
- **Security Hardened**: Cloudflare tunnels, YubiKey 2FA, network policies, and endpoint protection

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Kubernetes Cluster                       │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │   SLURM      │  │    Login     │  │   Compute Nodes  │  │
│  │  Controller  │  │     Node     │  │  (CPU/GPU/TPU)   │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  Slonklet    │  │    Health    │  │   Monitoring     │  │
│  │  Operator    │  │   Checks     │  │  (Prometheus)    │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### Components

- **SLURM Controller**: Cluster state management and job scheduling
- **Login Node**: User access point with SSH (via Cloudflare tunnel) and dev tools
- **Compute Nodes**: CPU, GPU (H100), and TPU nodes for job execution
- **Slonklet Operator**: Custom resource definitions for `PhysicalNode` and `SlurmJob`
- **Health System**: Automated monitoring (GPU burn-in, NCCL tests, disk checks, network validation)

## Quick Start

### Prerequisites

- Kubernetes 1.20+
- Helm 3.x
- External secret store (e.g., Google Secret Manager, AWS Secrets Manager)
- LDAP server (optional but recommended)
- Container registry access

### Installation

1. **Configure your environment**:
   ```bash
   git clone https://github.com/your-org/slonk.git
   cd slonk
   cp values-example.yaml values-custom.yaml
   # Edit values-custom.yaml with your settings (see Configuration section)
   ```

2. **Build container images**:
   ```bash
   # Base image
   cd containers/slonk && ./build.sh -t your-registry/slonk:latest

   # H100-specific image (optional)
   cd ../slonk-h100 && ./build.sh -t your-registry/slonk-h100:latest
   ```

3. **Deploy SLURM cluster**:
   ```bash
   helm install slurm charts/slurm/ -f values-custom.yaml -n slurm --create-namespace
   ```

4. **Verify deployment**:
   ```bash
   kubectl get pods -n slurm
   scontrol show nodes  # From login node
   ```

## Configuration

**Critical settings to customize** (see [CONFIGURATION.md](CONFIGURATION.md) for details):

| Setting | Location | Description |
|---------|----------|-------------|
| Container images | `values.yaml` | Update all `gcr.io/your-org/*` references |
| External secrets | `values.yaml` | Configure secret store and secret names |
| LDAP settings | `values.yaml` | LDAP server, user/group filters |
| Network IPs | `cluster-addons/slurm/*.yaml` | Cluster IPs, filestore IPs |
| API group | `config/crd/bases/*.yaml` | Replace `slonk.your-org.com` |
| Git repos | `values.yaml` | Update gitSyncRepos URLs |

**Example minimal configuration**:
```yaml
image: your-registry/slonk:latest
secrets:
  externalSecretStore: your-secret-store
  mungeKeyExternalSecretName: slurm-munge-key
ldap:
  uri: "ldaps://ldap.yourcompany.com"
  userBase: "ou=Users,dc=yourcompany,dc=com"
```

See [CONFIGURATION.md](CONFIGURATION.md) for comprehensive setup guide.

## Development

### Project Structure

```
slonk/
├── charts/slurm/          # Helm chart for SLURM cluster
│   ├── templates/         # K8s manifests (StatefulSets, ConfigMaps, CRDs)
│   ├── scripts/           # Shell scripts for prolog/epilog, health checks
│   └── values.yaml        # Default configuration
├── cluster-addons/        # Cluster-specific configs (ArgoCD ApplicationSets)
├── containers/            # Container build definitions
│   ├── slonk/            # Base SLURM container
│   └── slonk-h100/       # H100-optimized container
├── user/slonk/           # Python package and operator
│   ├── slonk/            # Python modules (health checks, lifecycle management)
│   └── operators/        # Kubernetes operator (Go)
└── CONFIGURATION.md      # Detailed configuration guide
```

### Building the Operator

```bash
cd user/slonk/operators/slonklet
make manifests  # Generate CRDs
make install    # Install CRDs to cluster
make run        # Run operator locally
```

### Adding Health Checks

Implement `slonk.health.base.HealthCheck` interface:

```python
from slonk.health.base import HealthCheck

class CustomHealthCheck(HealthCheck):
    def check(self):
        # Your health check logic
        # Raise exception on failure
        pass
```

Register in `slonk/health/__init__.py`.

## Monitoring

- **Metrics**: Prometheus endpoint on port 8071
- **Logs**:
  - SLURM: `/var/log/slurm/*.log`
  - Operator: `kubectl logs -n slurm deployment/slonklet-controller`
- **Custom Resources**: `kubectl get physicalnodes,slurmjobs -n slurm`

## Troubleshooting

| Issue | Check | Solution |
|-------|-------|----------|
| Pods not starting | `kubectl describe pod -n slurm` | Verify image pull secrets, node resources |
| SLURM nodes down | `scontrol show nodes` | Check slurmd logs, network connectivity |
| GPU allocation fails | `nvidia-smi` on compute node | Verify device plugin, driver versions |
| Auth failures | LDAP sync logs | Verify LDAP credentials, user filters |

**Debug commands**:
```bash
# Check cluster state
kubectl get all -n slurm
sinfo && squeue

# View logs
kubectl logs -n slurm -l app=slurm-controller --tail=100
kubectl logs -n slurm -l app=slurm-compute --tail=100

# Operator status
kubectl get physicalnodes -n slurm -o wide
kubectl describe physicalnode <node-name> -n slurm
```

## Security

- **Authentication**: LDAP + optional YubiKey 2FA
- **Network**: Cloudflare tunnel for SSH, Kubernetes network policies
- **Secrets**: External secret store integration (no secrets in Git)
- **Access Control**: RBAC for operator, SLURM accounts via LDAP

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new health checks or operator features
4. Submit a pull request

## License

See [LICENSE](LICENSE) file.

## Support

- Issues: [GitHub Issues](https://github.com/your-org/slonk/issues)
- Documentation: See [CONFIGURATION.md](CONFIGURATION.md) for setup details
- Example configs: `cluster-addons/slurm/` and `values-example.yaml`
