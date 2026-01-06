output "db_instance_host" {
  value = module.primary.db_instance_address
}

# App database (created by provision-psql.sh)
output "db_instance_name" {
  value = "customer_dashboard"
}

# App user with IAM auth (created by provision-psql.sh)
output "db_instance_username" {
  value = "customer_dashboard"
}

# Admin credentials - only for provisioning, not used by service
output "db_admin_username" {
  value     = module.primary.db_instance_username
  sensitive = true
}

output "db_admin_password" {
  value     = module.primary.db_instance_password
  sensitive = true
}
