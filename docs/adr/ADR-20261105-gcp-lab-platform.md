# ADR-20261105: GCP lab platform and ownership

- Status: Proposed; Terraform configuration is not applied.
- Context: Run the current microservice stack on GKE while keeping the existing GCP project unchanged and capping the lab by time and budget.
- Decision: Create a separately billed/labelled project, one-region GKE Standard with private nodes and Workload Identity, Cloud SQL PostgreSQL over private IP, Artifact Registry, Secret Manager, versioned storage, Cloud Logging/Monitoring, and a separately managed Cloudflare DNS zone.
- Ownership: Terraform owns project/network/cluster/cloud resources. Argo CD owns in-cluster application releases. Do not manage the same Kubernetes object from both. Cloudflare DNS stays manually reviewed until an API token and import plan are approved.
- Trade-offs: A two-node, zonal database, one broker, and one Milvus standalone are budget-limited lab topology, not HA. Private nodes depend on Cloud NAT. External dependencies/providers remain single-region.
- Security: GitHub Actions must use OIDC/WIF, not service-account JSON. Secret versions must be written by a local secure flow and mapped per workload; Terraform state must not contain secret material.
- Consequences: Actual billing and SKU estimates, quota, Secret Manager sync, CI promotion and the DNS cutover remain gates before apply/publication.
