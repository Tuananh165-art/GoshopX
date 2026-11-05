# GoshopX GCP lab teardown

**Run before the earliest of 48 hours after apply or 12 hours before the verified credit expiry.** The Terraform budget is an alert only. This runbook deletes only dedicated GoshopX lab resources. `generated-mote-467410-b0` is an existing project: never delete, disable billing for, or broadly modify the project itself. Keep unrelated project resources and the billing account untouched.

## 1. Confirm target and preserve needed data

```powershell
Set-Location F:\GoshopX\infra\gcp
$Project = terraform output -raw project_id
gcloud projects describe $Project --format="yaml(projectId,name,lifecycleState,labels)"
terraform state list
```

Stop unless the project ID is exactly `generated-mote-467410-b0` and the Terraform state contains only dedicated GoshopX resources. This project is pre-existing and has no GoshopX ownership labels. Export database backups, catalog snapshots, logs and any evidence to an approved destination. Confirm DNS rollback records are available and restore Cloudflare apex/`www` records to the saved prior targets before deleting the load balancer. Keep mail records unchanged.

## 2. Stop ingress and GitOps reconciliation

Revert/disable the public Gateway and HTTPRoutes in Git, then wait until GKE Gateway has removed forwarding rules, IP attachments and NEGs:

```powershell
kubectl get gateway,httproute -A
kubectl get svc,ingress -A
```

Confirm the public IP is no longer serving the application. Suspend Argo CD auto-sync for the lab application, then remove its finalizers only if the controller is unavailable and the manifests are understood. Delete application releases and wait for all pods, LoadBalancer services and PVC controllers to finish.

## 3. Remove Kubernetes data and platform resources

```powershell
kubectl delete application goshopx -n argocd --wait=true
helm uninstall goshopx -n goshopx --wait --timeout 10m
kubectl get pvc,pv -A
```

PVC deletion destroys Kafka, Redis, Milvus, Elasticsearch and local database data. Delete remaining PVCs only after the exports are verified. Check persistent disks, snapshots, forwarding rules, addresses, NEGs, firewall rules and Cloud NAT after GKE deletion. Cloud SQL is an external managed instance; check backup-retention requirements before Terraform destroys it.

## 4. Terraform destroy the lab

Inspect target one final time, then plan destruction:

```powershell
terraform plan -destroy -out destroy.tfplan
terraform show destroy.tfplan
```

Confirm the plan destroys only dedicated GoshopX resources. Do not destroy the shared billing account, Cloudflare zone, existing project, pre-existing buckets, networks, or IAM bindings. If state is stored in the GoshopX state bucket, first migrate it to a protected local backend and verify the migrated state before planning deletion. Apply only after the state and plan agree:

```powershell
terraform apply destroy.tfplan
gcloud projects describe $Project --format="value(lifecycleState)"
```

This Terraform configuration does not manage or delete the project. After destroy, inspect residual billable resources and remove only confirmed GoshopX-owned resources. Never use `gcloud projects delete` for this project as part of lab cleanup.

## 5. Verify residual billing and clean bootstrap state

Check the Cloud Console asset inventory for instances, disks, snapshots, static IPs, NAT gateways, buckets including old object versions, Artifact Registry images, Secret Manager versions, KMS keys, forwarding rules, DNS authorizations, certificates and IAM bindings. Empty versioned buckets before deletion; check object generations and retention locks. Delete the GCS Terraform state bucket last, only after the project is gone and the state has been archived according to policy. Remove local `.tfstate`, saved plans and credentials only after confirming no recovery is needed.

Review billing again after reporting delay (at least 24–48 hours), and keep the dated final inventory and cost report. `terraform destroy` exit code alone does not establish that all billable resources or delayed usage are gone.
