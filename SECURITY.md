# Security policy

This repository is an engineering prototype and is not a supported production distribution.

Report suspected vulnerabilities privately to the repository owner. Do not include kubeconfig files, service-account tokens, device serial numbers, tenant payloads, or private cluster logs in public issues.

The W1 deployment runs as a non-root user with a read-only root filesystem and dropped Linux capabilities. RBAC currently reuses the upstream `system:kube-scheduler` and `system:volume-scheduler` ClusterRoles plus the narrow extension-apiserver authentication reader Role; a project-specific least-privilege role is required before a production claim.
