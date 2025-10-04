# Cluster-Specific Configurations

ArgoCD ApplicationSet definitions for deploying Slonk to specific Kubernetes clusters.

## Structure

Each YAML file defines cluster-specific overrides for the base Helm chart:

- `slurm/nb0.yaml`: Example cluster configuration template

## Usage with ArgoCD

1. Deploy ApplicationSet to ArgoCD:
```bash
kubectl apply -f slurm/your-cluster.yaml -n argocd
```

2. ArgoCD will automatically sync the SLURM chart to target clusters based on the git selector.

## Configuration Pattern

ApplicationSets use the Helm plugin with `values-override` to customize per-cluster:

```yaml
parameters:
  - name: values-override
    string: |
      clusterName: my-cluster
      image: your-registry/slonk-h100:latest
      loginClusterIP: 10.96.4.16
      nodepools:
        h100:
          replicas: 80
```

## Adding New Clusters

1. Copy existing YAML file
2. Update `metadata.name` and cluster selector
3. Modify `values-override` with cluster-specific settings:
   - Cluster IP addresses
   - Node pool sizes and types
   - Storage configurations
   - Filestore/NFS mount points

## Key Customizations

- **Node pools**: Adjust replicas and resources for compute nodes
- **Network**: Set `loginClusterIP` and filestore IPs
- **Storage**: Configure volume mounts (`/data`, `/mnt/nfs/...`)
- **Tolerations**: Match node taints in your cluster
- **Images**: Use cluster-appropriate image (base vs H100)

## Best Practices

- Keep cluster configs in version control
- Use external secrets for sensitive data
- Test changes in dev clusters first
- Document cluster-specific requirements in comments
