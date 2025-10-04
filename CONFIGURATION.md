# Configuration Guide

This guide provides detailed instructions for configuring Slonk for your environment.

## Overview

Slonk requires several configuration changes before deployment. This guide walks through each section and what needs to be updated.

## 1. Container Images

### Update Image References

Replace all container image references with your registry:

**In `charts/slurm/values.yaml`:**
```yaml
image: gcr.io/your-org/slonk:latest
```

**In `cluster-addons/slurm/*.yaml`:**
```yaml
image: gcr.io/your-org/slonk-h100:latest
```

### Build Your Images

1. **Main Slonk Image**:
   ```bash
   cd containers/slonk
   ./build.sh
   ```

2. **H100-specific Image**:
   ```bash
   cd containers/slonk-h100
   ./build.sh
   ```

## 2. External Secrets Configuration

### Secret Store Setup

Configure your external secret store in `charts/slurm/values.yaml`:

```yaml
secrets:
  externalSecretStore: your-external-secret-store
  crowdstrikeCredsExternalSecretName: your-crowdstrike-creds-secret
  idRsaClusterExternalSecretName: your-ssh-keys-secret
  ldapClientCredsExternalSecretName: your-ldap-client-creds-secret
  mungeKeyExternalSecretName: your-munge-key-secret
  yubikeyPamExternalSecretName: your-yubikey-pam-secret
```

### Required Secrets

You need to create the following secrets in your external secret store:

1. **Crowdstrike Credentials**: For endpoint protection
2. **SSH Keys**: For cluster node communication
3. **LDAP Client Credentials**: For user authentication
4. **Munge Key**: For SLURM authentication
5. **YubiKey PAM**: For 2FA (optional)

## 3. LDAP Configuration

### Update LDAP Settings

In `charts/slurm/values.yaml`:

```yaml
ldap:
  uri: "ldaps://your-ldap-server.com"
  userBase: "ou=Users,dc=yourcompany,dc=com"
  userFilter: "memberOf=cn=engineering,ou=Groups,dc=yourcompany,dc=com"
  groupBase: "dc=yourcompany,dc=com"
  groupFilter: "|(cn=engineering)(cn=security)(cn=developers)(memberOf=cn=engineering,ou=Groups,dc=yourcompany,dc=com)"
  commonGid: "1000"
```

### LDAP Bind Credentials

Update the LDAP bind credentials in `charts/slurm/templates/configmaps.yaml`:

```bash
# Replace these placeholders with your actual values
ldap_bind_dn = "REPLACEME-LDAP-BIND-DN"  # TODO: Replace with your LDAP bind DN
ldap_bind_password = "REPLACEME-LDAP-BIND-PASSWORD"  # TODO: Replace with your LDAP bind password
```

## 4. Network Configuration

### Cluster IP Addresses

Update IP addresses in `cluster-addons/slurm/*.yaml`:

**Example for cluster configurations:**
```yaml
loginClusterIP: 10.96.4.16  # TODO: Replace with your cluster IP
```

**Another example:**
```yaml
loginClusterIP: 10.43.4.14  # TODO: Replace with your cluster IP
```

### Filestore IP Addresses

Update filestore configurations:

```yaml
filestore:
  ip: 172.23.35.66  # TODO: Replace with your filestore IP
  volumeHandle: "modeInstance/us-central1/your-k8s-cluster-filestore/home"
```

### Data Mount IPs

Update data mount IPs in `charts/slurm/templates/configmaps.yaml`:

```bash
mount -o rw,intr,nolock 10.121.6.2:/data /data  # TODO: Replace with your data mount IP
```

## 5. Cloudflare Configuration

### Tunnel Setup

If using Cloudflare for SSH access:

```yaml
cloudflare:
  enabled: true
  externalSecretStore: your-external-secret-store
  tunnelExternalSecretName: your-cloudflare-tunnel-secret
  shortLivedCertExternalSecretName: your-cloudflare-cert-secret
```

### Required Cloudflare Secrets

1. **Tunnel Token**: For Cloudflare tunnel setup
2. **Short-lived Certificates**: For SSH certificate authentication

## 6. Storage Configuration

### Storage Classes

Update storage class names in your cluster configurations:

```yaml
storageClassName: "premium-rwo"  # Replace with your storage class
storageClassName: "enterprise-rwx"  # Replace with your storage class
```

### Persistent Volumes

Configure persistent volume settings:

```yaml
volumeClaimTemplates:
  - metadata:
      name: slurm-controller-pvc
    spec:
      accessModes: ["ReadWriteOnce"]
      storageClassName: "your-storage-class"
      resources:
        requests:
          storage: 100Gi
```

## 7. Git Repository Configuration

### Update Repository URLs

In `charts/slurm/values.yaml`:

```yaml
gitSyncRepos:
  - name: k8s
    externalSecretName: your-git-creds-secret
    repo: git@github.com:your-org/your-k8s-repo.git
    branch: main
    destination: /home/common/git-sync/k8s
    wait: 30
    timeout: 600
```

### Git Credentials

Create a secret with your Git credentials:

```bash
kubectl create secret generic your-git-creds-secret \
  --from-file=ssh-privatekey=~/.ssh/id_rsa \
  --from-file=ssh-knownhosts=~/.ssh/known_hosts
```

## 8. Health Check Configuration

### Update Health Check IPs

In `user/slonk/slonk/health/weka.py`:

```python
# Replace with your actual IP addresses
bash("timeout 1.0 ping -c 1 172.20.7.141")  # TODO: Replace with your ceph cluster IPs
bash("timeout 1.0 ping -c 1 172.20.7.142")  # TODO: Replace with your ceph cluster IPs
bash("timeout 1.0 ping -c 1 172.20.7.159")  # TODO: Replace with your weka cluster IP
```

### Custom Health Checks

Add your own health checks in `user/slonk/slonk/health/`:

```python
from slonk.health.base import HealthCheck

class YourCustomHealthCheck(HealthCheck):
    def check(self):
        # Your health check logic here
        pass
```

## 9. Kubernetes Operator Configuration

### Update API Group

The operator uses custom resources. Update the API group in Go files:

```go
// In all Go files, replace the API group:
"slonk.example.com" -> "slonk.your-org.com"
```

### Update Import Paths

Update all import statements in Go files:

```go
// Replace import paths:
"github.com/example/slonklet" -> "github.com/your-org/slonklet"
```

## 10. Cluster-Specific Configurations

### Node Pool Configuration

Update node pool specifications in `cluster-addons/slurm/*.yaml`:

```yaml
nodepools:
  controller:
    replicas: 1
    resources:
      requests:
        cpu: 8
        memory: 32Gi
  login:
    replicas: 1
    resources:
      requests:
        cpu: 62
        memory: 220Gi
  cpu:
    replicas: 0  # Adjust based on your needs
    slurmConfig:
      CPUs: 16
      RealMemory: 65536
      Features: ["cpu"]
```

### GPU Configuration

For GPU nodes:

```yaml
gscConfigs:
  h100:
    nodepoolPrefix: h100
    numSlices: 5
    replicasPerSlice: 80
gscCommonNodeTemplate:
  slurmConfig:
    CPUs: 96
    RealMemory: "1048576"
    Features: ["h100"]
    Gres: "gpu:8"
  resources:
    limits:
      cpu: 96
      memory: 1024Gi
      nvidia.com/gpu: 8
```

## 11. Security Configuration

### Network Policies

Create network policies for your cluster:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: slurm-network-policy
  namespace: slurm
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: slurm
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
```

### RBAC Configuration

Update RBAC rules in `charts/slurm/templates/serviceaccount.yaml`:

```yaml
- apiGroups:
  - slonk.your-org.com
  resources:
  - physicalnodes
  - slurmjobs
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
```

## 12. Monitoring Configuration

### Prometheus Integration

Configure Prometheus monitoring:

```yaml
# In your values file
monitoring:
  enabled: true
  prometheus:
    enabled: true
    port: 8071
```

### Logging Configuration

Configure logging for different components:

```yaml
# SLURM logging
slurm:
  SlurmctldLogFile: /var/log/slurm/slurmctld.log
  SlurmdLogFile: /var/log/slurm/slurmd.log

# Application logging
logging:
  level: info
  format: json
```

## 13. Backup and Disaster Recovery

### Backup Configuration

Set up backup for critical data:

```yaml
backup:
  enabled: true
  schedule: "0 2 * * *"  # Daily at 2 AM
  retention: 30  # days
  storage:
    type: gcs
    bucket: your-backup-bucket
```

## 14. Validation Checklist

Before deploying, ensure you have:

### Critical Configuration
- [ ] Updated all container image references (`gcr.io/your-org/` → your registry)
- [ ] Configured external secrets and secret stores
- [ ] Updated LDAP configuration (server, user base, group filters)
- [ ] Replaced all hardcoded IP addresses with your cluster IPs
- [ ] Updated storage configurations and mount points
- [ ] Configured Cloudflare tunnel (if using for SSH access)
- [ ] Updated Git repository URLs to your organization's repos
- [ ] Replaced API group references (`slonk.your-org.com` → your domain)

### Security and Authentication
- [ ] Set up SSH key management for cluster communication
- [ ] Configured LDAP bind credentials and client certificates
- [ ] Set up Munge key for SLURM authentication
- [ ] Configured YubiKey PAM (if using 2FA)
- [ ] Set up Crowdstrike endpoint protection (if using)

### Infrastructure and Networking
- [ ] Created appropriate network policies for your cluster
- [ ] Configured persistent volume storage classes
- [ ] Updated node pool configurations for your infrastructure
- [ ] Updated health check IP addresses and endpoints
- [ ] Set up Prometheus monitoring and alerting

### Testing and Validation
- [ ] Tested health checks with your infrastructure
- [ ] Verified network connectivity between components
- [ ] Validated SLURM configuration and job submission
- [ ] Tested authentication and user access
- [ ] Verified backup and disaster recovery procedures

## 15. Testing Your Configuration

### Pre-deployment Tests

1. **Validate Helm Charts**:
   ```bash
   helm lint charts/slurm/
   ```

2. **Test Configuration**:
   ```bash
   helm template slurm charts/slurm/ -f your-values.yaml
   ```

3. **Validate Secrets**:
   ```bash
   kubectl get secrets -n slurm
   ```

### Post-deployment Verification

1. **Check SLURM Status**:
   ```bash
   scontrol ping
   scontrol show nodes
   ```

2. **Verify Health Checks**:
   ```bash
   kubectl get physicalnodes -n slurm
   ```

3. **Test Authentication**:
   ```bash
   ssh user@login-node
   ```

## Troubleshooting

### Common Configuration Issues

1. **Image Pull Errors**: Verify image references and registry access
2. **Secret Not Found**: Check external secret store configuration
3. **LDAP Connection Failures**: Verify LDAP server connectivity and credentials
4. **Network Connectivity**: Check IP addresses and firewall rules
5. **Storage Issues**: Verify storage class names and permissions

### Debug Commands

```bash
# Check pod status
kubectl get pods -n slurm

# View logs
kubectl logs -n slurm deployment/slurm-controller

# Check events
kubectl get events -n slurm

# Verify SLURM configuration
scontrol show config
```

## Support

For configuration issues:
1. Check the troubleshooting section
2. Review logs for error messages
3. Verify all required secrets are present
4. Test network connectivity
5. Validate Kubernetes resources 