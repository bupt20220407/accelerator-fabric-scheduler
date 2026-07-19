# Version baseline

| Component | Locked value | Rationale |
|---|---|---|
| Kubernetes modules | `v1.35.5` / `v0.35.5` | One minor and patch across scheduler staging modules |
| Go | `1.25.5` | Patch release in the Kubernetes 1.35 supported Go line |
| Go image | `golang:1.25.5-bookworm@sha256:d9132cce84391efab786495288756d60e1da215b1f94e87860aeefc3d4c45b6d` | Reproducible local and image builds |
| kind | `v0.32.0` | Verified local CLI version |
| kind node | `kindest/node@sha256:ce977ae6d65918d0b58a5f8b5e940429c2ce42fa3a5619ec2bbc60b949c0ac95` (`v1.35.5`) | Verified from the image's `kubeadm version` |
| kubectl | `v1.36.2` client | One-minor skew from the v1.35.5 cluster |
| scheduler-plugins | `v0.35.4-devel` candidate, not yet imported | Phase 2 Coscheduling integration only |
| CRD API | `scheduling.bupt.dev/v1alpha1` | Experimental API; breaking changes are still allowed |
| Custom scheduler binary | `v1.35.5-accelerator.0.1.0` | Kubernetes base plus project pre-release identity |
| Kubernetes code-generator | `v0.35.5` | Matches the API/client-go patch baseline |
| Device Plugin API | `v1beta1` from `k8s.io/kubelet v0.35.5` | Current kubelet registration and Allocate contract |

Do not update a single Kubernetes module independently. Version upgrades require a dedicated change that runs unit tests, image build, kind smoke, and CRD server-side validation.
