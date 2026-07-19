# Security policy

This repository is an engineering prototype and is not a supported production distribution.

Report suspected vulnerabilities privately to the repository owner. Do not include kubeconfig files, service-account tokens, device serial numbers, tenant payloads, or private cluster logs in public issues.

The scheduler and topology controller run as non-root users with read-only root filesystems and dropped Linux capabilities. Scheduler RBAC currently reuses the upstream `system:kube-scheduler` and `system:volume-scheduler` ClusterRoles plus the narrow extension-apiserver authentication reader Role; a project-specific least-privilege role is required before a production claim.

The synthetic Device Plugin runs as UID 0 because kubelet's host-mounted device-plugin directory is root-owned. It is not privileged, drops all Linux capabilities, has a read-only root filesystem, and mounts only `/var/lib/kubelet/device-plugins`. It does not mount host devices or vendor runtimes. This exception is for the kind fixture path and must not be presented as a hardened production deployment.

The synthetic DRA driver also runs as UID 0 and has broader writable hostPath access to `/var/lib/kubelet/plugins`, `/var/lib/kubelet/plugins_registry`, and `/var/run/cdi`. Those mounts are required for kubelet registration, restart-persistent synthetic claim state, and CDI injection. The container is not privileged, drops all capabilities, and uses a read-only root filesystem, but a compromised driver could disrupt node resource plugins or CDI configuration. Its RBAC is limited to reading Nodes and ResourceClaims and managing ResourceSlices. This deployment is suitable only for the isolated kind fixture environment until hostPath isolation, admission policy, image signing, and vendor-specific threat analysis are completed.
