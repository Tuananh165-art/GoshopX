terraform {
  required_version = ">= 1.10, < 2.0"
  required_providers {
    google = { source = "hashicorp/google", version = "~> 7.0" }
  }
}

provider "google" {
  project               = var.project_id
  region                = var.region
  zone                  = var.zone
  billing_project       = var.project_id
  user_project_override = true
}

data "google_project" "target" {
  project_id = var.project_id
}

resource "google_project_service" "apis" {
  for_each = toset([
    "artifactregistry.googleapis.com", "compute.googleapis.com", "container.googleapis.com",
    "iam.googleapis.com", "logging.googleapis.com", "monitoring.googleapis.com",
    "secretmanager.googleapis.com", "serviceusage.googleapis.com", "sqladmin.googleapis.com",
    "storage.googleapis.com", "cloudresourcemanager.googleapis.com", "servicenetworking.googleapis.com",
    "binaryauthorization.googleapis.com", "cloudkms.googleapis.com", "certificatemanager.googleapis.com",
    "billingbudgets.googleapis.com"
  ])
  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

resource "google_compute_network" "lab" {
  name                    = "goshopx-lab"
  auto_create_subnetworks = false
  depends_on              = [google_project_service.apis]
}

resource "google_compute_subnetwork" "gke" {
  name                     = "goshopx-gke-${var.region}"
  region                   = var.region
  network                  = google_compute_network.lab.id
  ip_cidr_range            = "10.20.0.0/20"
  private_ip_google_access = true
  secondary_ip_range {
    range_name    = "gke-pods"
    ip_cidr_range = "10.24.0.0/14"
  }
  secondary_ip_range {
    range_name    = "gke-services"
    ip_cidr_range = "10.28.0.0/20"
  }
}

resource "google_compute_router" "lab" {
  name    = "goshopx-nat-router"
  region  = var.region
  network = google_compute_network.lab.id
}

resource "google_compute_router_nat" "lab" {
  name                               = "goshopx-nat"
  router                             = google_compute_router.lab.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "LIST_OF_SUBNETWORKS"
  subnetwork {
    name                    = google_compute_subnetwork.gke.id
    source_ip_ranges_to_nat = ["ALL_IP_RANGES"]
  }
  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }
}

resource "google_compute_global_address" "sql_private" {
  name          = "goshopx-sql-private-range"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = google_compute_network.lab.id
}

resource "google_service_networking_connection" "private_vpc" {
  network                 = google_compute_network.lab.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.sql_private.name]
}

resource "google_container_cluster" "lab" {
  name              = "goshopx-lab"
  location          = var.zone
  network           = google_compute_network.lab.id
  subnetwork        = google_compute_subnetwork.gke.id
  networking_mode   = "VPC_NATIVE"
  datapath_provider = "ADVANCED_DATAPATH"
  gateway_api_config { channel = "CHANNEL_STANDARD" }
  remove_default_node_pool = true
  initial_node_count       = 1
  deletion_protection      = false
  release_channel { channel = "REGULAR" }
  ip_allocation_policy {
    cluster_secondary_range_name  = "gke-pods"
    services_secondary_range_name = "gke-services"
  }
  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = false
    master_ipv4_cidr_block  = "172.16.0.0/28"
  }
  workload_identity_config { workload_pool = "${var.project_id}.svc.id.goog" }
  addons_config {
    http_load_balancing { disabled = false }
    gke_backup_agent_config { enabled = false }
  }
  logging_config { enable_components = ["SYSTEM_COMPONENTS", "WORKLOADS"] }
  monitoring_config { enable_components = ["SYSTEM_COMPONENTS", "APISERVER", "SCHEDULER", "CONTROLLER_MANAGER"] }
  depends_on = [google_project_service.apis, google_compute_router_nat.lab]
}

resource "google_container_node_pool" "lab" {
  name       = "goshopx-lab-pool"
  location   = var.zone
  cluster    = google_container_cluster.lab.name
  node_count = var.node_count
  autoscaling {
    min_node_count = 1
    max_node_count = 2
  }
  management {
    auto_repair  = true
    auto_upgrade = true
  }
  node_config {
    machine_type    = var.machine_type
    disk_type       = "pd-balanced"
    disk_size_gb    = 80
    image_type      = "COS_CONTAINERD"
    service_account = google_service_account.gke_nodes.email
    oauth_scopes    = ["https://www.googleapis.com/auth/cloud-platform"]
    workload_metadata_config { mode = "GKE_METADATA" }
    shielded_instance_config {
      enable_secure_boot          = true
      enable_integrity_monitoring = true
    }
    labels = { application = "goshopx", purpose = "lab" }
  }
}

resource "google_service_account" "gke_nodes" {
  account_id   = "goshopx-gke-nodes"
  display_name = "GoshopX GKE node identity"
}

resource "google_project_iam_member" "node_registry_read" {
  project = var.project_id
  role    = "roles/artifactregistry.reader"
  member  = "serviceAccount:${google_service_account.gke_nodes.email}"
}

resource "google_artifact_registry_repository" "images" {
  location      = var.region
  repository_id = "goshopx"
  description   = "GoshopX lab container images"
  format        = "DOCKER"
  depends_on    = [google_project_service.apis]
}

resource "google_iam_workload_identity_pool" "github" {
  workload_identity_pool_id = "goshopx-github"
  display_name              = "GoshopX GitHub Actions"
  description               = "GitHub OIDC for the main branch release workflow"
  depends_on                = [google_project_service.apis]
}

resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "main-release"
  display_name                       = "GoshopX main release"
  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
    "attribute.ref"        = "assertion.ref"
  }
  attribute_condition = "assertion.repository == '${var.github_repository}' && assertion.ref == 'refs/heads/main'"
  oidc { issuer_uri = "https://token.actions.githubusercontent.com" }
}

resource "google_service_account" "github_release" {
  account_id   = "goshopx-github-release"
  display_name = "GoshopX GitHub release publisher"
}

resource "google_artifact_registry_repository_iam_member" "github_writer" {
  location   = google_artifact_registry_repository.images.location
  repository = google_artifact_registry_repository.images.name
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.github_release.email}"
}

resource "google_service_account_iam_member" "github_release_oidc" {
  service_account_id = google_service_account.github_release.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/${var.github_repository}"
}

resource "google_storage_bucket" "catalog" {
  name                        = "${var.project_id}-catalog"
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  versioning { enabled = true }
  lifecycle_rule {
    condition { age = 30 }
    action { type = "Delete" }
  }
}

resource "google_storage_bucket" "milvus" {
  name                        = "${var.project_id}-milvus"
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  versioning { enabled = true }
}

resource "google_storage_bucket" "media" {
  name                        = "${var.project_id}-media"
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
}

resource "google_storage_bucket" "tfstate" {
  name                        = "${var.project_id}-tfstate"
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  versioning { enabled = true }
}

resource "google_storage_bucket" "loki" {
  name                        = "${var.project_id}-loki"
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  lifecycle_rule {
    condition { age = 3 }
    action { type = "Delete" }
  }
}

resource "google_sql_database_instance" "postgres" {
  name                = "goshopx-postgres"
  region              = var.region
  database_version    = "POSTGRES_16"
  deletion_protection = false
  settings {
    edition           = "ENTERPRISE"
    tier              = "db-custom-1-3840"
    availability_type = "ZONAL"
    disk_type         = "PD_SSD"
    disk_size         = 20
    disk_autoresize   = false
    ip_configuration {
      ipv4_enabled    = false
      private_network = google_compute_network.lab.id
    }
    backup_configuration {
      enabled                        = true
      point_in_time_recovery_enabled = false
    }
    database_flags {
      name  = "max_connections"
      value = "100"
    }
  }
  depends_on = [google_service_networking_connection.private_vpc]
}

locals {
  databases = toset(["account", "order", "payment", "inventory", "notification", "admin", "recommender"])
  runtime_secrets = toset([
    "DEEPSEEK_API_KEY", "GEMINI_API_KEY", "TAVILY_API_KEY", "TYPESAFE_API_KEY", "LANGSMITH_API_KEY",
    "JWT_SECRET", "VNPAY_HASH_SECRET", "SMTP_PASSWORD", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY",
    "ACCOUNT_DATABASE_URL", "PRODUCT_DATABASE_URL", "ORDER_DATABASE_URL", "PAYMENT_DATABASE_URL",
    "INVENTORY_DATABASE_URL", "NOTIFICATION_DATABASE_URL", "ADMIN_DATABASE_URL", "RECOMMENDER_DATABASE_URL",
    "GOOGLE_CLIENT_ID", "VNPAY_TMN_CODE", "VNPAY_PAYMENT_URL", "GMAIL_USERNAME", "GMAIL_FROM",
  ])
}

resource "google_sql_database" "domain" {
  for_each = local.databases
  name     = each.key
  instance = google_sql_database_instance.postgres.name
}

resource "google_secret_manager_secret" "runtime" {
  for_each  = local.runtime_secrets
  secret_id = "goshopx-${lower(replace(each.key, "_", "-"))}"
  replication {
    auto {}
  }
  depends_on = [google_project_service.apis]
}

resource "google_service_account" "runtime" {
  account_id   = "goshopx-runtime"
  display_name = "GoshopX workload identity"
}

resource "google_project_iam_member" "runtime_sql" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_secret_manager_secret_iam_member" "runtime_secrets" {
  for_each  = google_secret_manager_secret.runtime
  project   = var.project_id
  secret_id = each.value.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_storage_bucket_iam_member" "recommender_catalog" {
  bucket = google_storage_bucket.catalog.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_storage_bucket_iam_member" "runtime_milvus" {
  bucket = google_storage_bucket.milvus.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_storage_bucket_iam_member" "runtime_media_creator" {
  bucket = google_storage_bucket.media.name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_storage_bucket_iam_member" "runtime_media_reader" {
  bucket = google_storage_bucket.media.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.runtime.email}"
}

resource "google_service_account_iam_member" "runtime_workload_identity" {
  service_account_id = google_service_account.runtime.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[goshopx/goshopx-runtime]"
  depends_on         = [google_container_cluster.lab]
}

resource "google_compute_global_address" "https" {
  name = "goshopx-https"
}

resource "google_certificate_manager_dns_authorization" "apex" {
  name   = "goshopx-apex"
  domain = var.domain
  depends_on = [
    google_project_service.apis,
  ]
}

resource "google_certificate_manager_dns_authorization" "www" {
  name   = "goshopx-www"
  domain = "www.${var.domain}"
  depends_on = [
    google_project_service.apis,
  ]
}

resource "google_certificate_manager_certificate" "apex" {
  name = "goshopx-apex"
  managed {
    domains = [var.domain, "www.${var.domain}"]
    dns_authorizations = [
      google_certificate_manager_dns_authorization.apex.id,
      google_certificate_manager_dns_authorization.www.id,
    ]
  }
}

resource "google_certificate_manager_certificate_map" "https" {
  name       = "goshopx-map"
  depends_on = [google_project_service.apis]
}

resource "google_certificate_manager_certificate_map_entry" "apex" {
  name         = "goshopx-apex-entry"
  map          = google_certificate_manager_certificate_map.https.name
  hostname     = var.domain
  certificates = [google_certificate_manager_certificate.apex.id]
}

resource "google_certificate_manager_certificate_map_entry" "www" {
  name         = "goshopx-www-entry"
  map          = google_certificate_manager_certificate_map.https.name
  hostname     = "www.${var.domain}"
  certificates = [google_certificate_manager_certificate.apex.id]
}

resource "google_billing_budget" "lab" {
  billing_account = var.billing_account
  display_name    = "GoshopX 48h lab budget"
  budget_filter { projects = ["projects/${data.google_project.target.number}"] }
  amount {
    specified_amount {
      currency_code = "VND"
      units         = "2500000"
    }
  }
  threshold_rules { threshold_percent = 0.5 }
  threshold_rules { threshold_percent = 0.75 }
  threshold_rules { threshold_percent = 0.9 }
  threshold_rules { threshold_percent = 1.0 }
}
