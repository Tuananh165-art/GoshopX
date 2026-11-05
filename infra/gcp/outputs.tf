output "project_id" { value = var.project_id }
output "cluster_name" { value = google_container_cluster.lab.name }
output "region" { value = var.region }
output "zone" { value = var.zone }
output "artifact_registry" { value = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.images.repository_id}" }
output "https_ip" { value = google_compute_global_address.https.address }
output "certificate_dns_records" {
  value = {
    apex = google_certificate_manager_dns_authorization.apex.dns_resource_record
    www  = google_certificate_manager_dns_authorization.www.dns_resource_record
  }
}
output "runtime_service_account" { value = google_service_account.runtime.email }
output "sql_private_ip" { value = google_sql_database_instance.postgres.private_ip_address }
output "sql_connection_name" { value = google_sql_database_instance.postgres.connection_name }
output "github_wif_provider" { value = google_iam_workload_identity_pool_provider.github.name }
output "github_release_service_account" { value = google_service_account.github_release.email }
output "tfstate_bucket" { value = google_storage_bucket.tfstate.name }
