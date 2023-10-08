
output "app_id" {
  value = azuread_application.this.application_id
}

output "app_secret" {
  value     = azuread_service_principal_password.this.value
  sensitive = true
}