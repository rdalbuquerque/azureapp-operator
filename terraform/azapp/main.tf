terraform {
  required_providers {
    azurerm = {
      source = "hashicorp/azurerm"
    }
    azuread = {
      source = "hashicorp/azuread"
    }
  }
}

data "azurerm_client_config" "this" {}

data "azuread_client_config" "this" {}

data "azurerm_resource_group" "backend" {
  name = "terraformcloud-test-prd"
}

provider "azurerm" {
  features {}
}

resource "random_uuid" "roles" {
  for_each = toset(var.app_roles)
}

locals {
  app_roles = { for r in var.app_roles : r => { role_value = r, id = random_uuid.roles[r].result } }
}

resource "azuread_application" "this" {
  display_name    = var.display_name
  identifier_uris = [var.identifier_uri]
  dynamic "app_role" {
    for_each = local.app_roles
    content {
      display_name         = app_role.value.role_value
      value                = app_role.value.role_value
      id                   = app_role.value.id
      description          = "A role named ${app_role.value.role_value}"
      allowed_member_types = ["User", "Application"]
    }
  }
}

resource "azuread_application_password" "this" {
  application_object_id = azuread_application.this.object_id
}

resource "azuread_service_principal" "this" {
  application_id = azuread_application.this.application_id
}

resource "azurerm_key_vault" "this" {
  resource_group_name       = data.azurerm_resource_group.backend.name
  location                  = data.azurerm_resource_group.backend.location
  name                      = var.kv_name
  tenant_id                 = data.azurerm_client_config.this.tenant_id
  sku_name                  = "standard"
  enable_rbac_authorization = true
}

resource "azurerm_role_assignment" "current_client_2_kv" {
  scope                = azurerm_key_vault.this.id
  role_definition_name = "Key Vault Administrator"
  principal_id         = data.azuread_client_config.this.object_id
}
