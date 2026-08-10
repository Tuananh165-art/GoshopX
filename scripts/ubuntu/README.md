# Ubuntu K3s deployment scripts

For the full Vietnamese operator guide, use
[`docs/23-ubuntu-k3s-step-by-step-deployment.md`](../../docs/23-ubuntu-k3s-step-by-step-deployment.md).

Run these scripts on the Ubuntu VPS from a checked-out GoshopX repository.
They require a sudo-capable user, public DNS/TLS readiness if ingress is
enabled, and a separate secrets file that is never committed.

Create the file outside the checkout from `goshopx-secrets.env.example`, then
use `chmod 600 /secure/path/goshopx-secrets.env`. Do not put actual credentials
in the example file, GitHub variables, Helm values, shell history, or tickets.

1. `00-host-prerequisites.sh`
2. `01-install-k3s.sh`
3. `02-install-cli-tools.sh`
4. `03-create-runtime-secret.sh /secure/path/goshopx-secrets.env`
5. `04-bootstrap-platform.sh`
6. `08-build-and-push-images.sh` (optional; CI is preferred)
7. `05-deploy-application.sh`
8. `06-verify-release.sh`

Use `ENABLE_INFRA=true ./05-deploy-application.sh` only after checking node
headroom. It deploys the single-node Compose-matched dependency profile and is
not HA. The default profile leaves expensive data/AI workloads disabled.

`07-rollback-application.sh` rolls Helm back for emergency recovery. Revert the
GitOps promotion commit afterwards; Git must remain the desired-state source.

Do not enable dashboard NodePorts on an Internet-facing VPS. Use `kubectl
port-forward` or firewall-restricted operator access instead.

Run `chmod +x scripts/ubuntu/*.sh` once after cloning on Ubuntu.
