# SLURM Helm Chart

Helm chart for deploying a complete SLURM cluster on Kubernetes with support for CPU, GPU, and TPU compute nodes.

## Installation

```bash
helm install slurm . -f values.yaml -n slurm --create-namespace
```

## Chart Structure

### Templates

- `crd-*.yaml`: Custom resource definitions for PhysicalNode and SlurmJob
- `configmaps.yaml`: SLURM configuration, scripts, and setup logic
- `externalsecrets.yaml`: External secret bindings (munge key, SSH keys, LDAP creds)
- `nodepools.yaml`: StatefulSets for controller, login, and compute nodes
- `services.yaml`: Kubernetes services for SLURM components
- `serviceaccount.yaml`: RBAC for operator and pods
- `servicemonitor.yaml`: Prometheus monitoring configuration
- `pvc.yaml`: Persistent volume templates

### Scripts

Located in `scripts/` and mounted into pods:

- `k8s/`: Kubernetes-specific scripts (LDAP sync, metrics, SSH key management)
- `slurm/`: SLURM hooks (prolog, epilog, task_prolog, task_epilog)
- `profile.d/`: Login shell customizations
- `gpu/`: GPU-specific prolog/epilog scripts

## Configuration

### Node Pools

Define in `values.yaml` under `nodepools`:

```yaml
nodepools:
  controller:
    replicas: 1
    resources: { requests: { cpu: 8, memory: 32Gi } }
  login:
    replicas: 1
    isLoginNode: true
  h100:
    replicas: 5
    isSlurmComputeNode: true
    slurmConfig:
      CPUs: 96
      RealMemory: "819200"
      Gres: "gpu:8"
    resources:
      limits: { cpu: 96, memory: 1500Gi, nvidia.com/gpu: 8 }
```

### TPU Configuration

Configure TPU slices in `tpuConfigs`:

```yaml
tpuConfigs:
  v4-8:
    podType: tpu-v4-podslice
    numSlices: 16
    topology: 2x2x1
```

### Secrets

Required external secrets (configure in `values.yaml`):
- `mungeKeyExternalSecretName`: SLURM authentication
- `idRsaClusterExternalSecretName`: SSH keys for inter-node communication
- `ldapClientCredsExternalSecretName`: LDAP bind credentials
- `yubikeyPamExternalSecretName`: YubiKey 2FA (optional)
- `crowdstrikeCredsExternalSecretName`: Endpoint protection (optional)

## Key Features

- **Dynamic node pools**: Scale compute nodes via StatefulSet replicas
- **Health monitoring**: Automated health checks with remediation
- **Multi-architecture**: CPU, GPU (H100), TPU support
- **Persistent storage**: Controller state on PVC, shared NFS for home dirs
- **Cloudflare tunnel**: Secure SSH access to login node
- **LDAP integration**: User/group synchronization
- **Prometheus metrics**: Job and node metrics on port 8071

## Customization

1. **SLURM configuration**: Edit `configmaps.yaml` slurm.conf section
2. **Node resources**: Adjust CPU/memory limits in `nodepools`
3. **Storage**: Update `filestore` or `volumeClaimTemplates`
4. **Scripts**: Modify files in `scripts/` for custom setup logic

## Upgrading

```bash
helm upgrade slurm . -f values.yaml -n slurm
```

**Note**: Some resources (PVCs, secrets) may require manual intervention during upgrades.

## Uninstallation

```bash
helm uninstall slurm -n slurm
# Manually delete PVCs if needed
kubectl delete pvc -n slurm -l app=slurm-controller
```
