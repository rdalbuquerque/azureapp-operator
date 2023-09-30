
output "app_id" {
  value = azuread_application.this.application_id
}

output "app_secret" {
  value     = azuread_application_password.this.value
  sensitive = true
}