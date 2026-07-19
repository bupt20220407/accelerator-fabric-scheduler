# Security policy

This repository is an engineering prototype and is not a supported production distribution.

Report suspected vulnerabilities privately to the repository owner. Do not include kubeconfig files, service-account tokens, device serial numbers, tenant payloads, or private cluster logs in public issues.

The scheduler and topology controller run as non-root users with read-only root filesystems and dropped Linux capabilities. Scheduler RBAC currently reuses the upstream `system:kube-scheduler` and `system:volume-scheduler` ClusterRoles plus the narrow extension-apiserver authentication reader Role; a project-specific least-privilege role is required before a production claim.

The synthetic Device Plugin runs as UID 0 because kubelet's host-mounted device-plugin directory is root-owned. It is not privileged, drops all Linux capabilities, has a read-only root filesystem, and mounts only `/var/lib/kubelet/device-plugins`. It does not mount host devices or vendor runtimes. This exception is for the kind fixture path and must not be presented as a hardened production deployment.
