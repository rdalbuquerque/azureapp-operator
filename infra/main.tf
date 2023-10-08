terraform {
  cloud {
    organization = "example-org-cdf940"
    workspaces {
      name = "k8soperator"
    }
  }
  required_providers {
    azurerm = {}
  }
}

provider "azurerm" {
  features {
    key_vault {
      purge_soft_delete_on_destroy = false
    }
  }
  skip_provider_registration = false
}

locals {
  managedAt            = "github.com/rdalbuquerque/azureapp-operator/infra"
  common_resource_name = "rdak8soperator1"
}

data "azurerm_client_config" "current" {}

data "azurerm_resource_group" "operator" {
  name = var.resource_group_name
}

data "azuread_service_principal" "admin" {
  display_name = "terraform_agent"
}

resource "azuread_application" "operator_agent" {
  display_name = "k8soperator_agent" 
}

resource "azuread_service_principal" "operator_agent" {
  application_id = azuread_application.operator_agent.application_id 
}

# role assignment to allow operator agent to deleted vaults
resource "azurerm_role_assignment" "operator_agent_2_subscription" {
  principal_id         = azuread_service_principal.operator_agent.object_id
  role_definition_name = "Contributor"
  scope                = data.azurerm_client_config.current.subscription_id
}

resource "azurerm_storage_account" "operator" {
  name                     = "${local.common_resource_name}${var.env}"
  resource_group_name      = var.resource_group_name
  location                 = var.default_location
  account_tier             = "Standard"
  account_replication_type = "LRS"

  tags = {
    environment = var.env
    managedAt   = local.managedAt
  }
}

resource "azurerm_storage_container" "operator" {
  name                  = "state"
  storage_account_name  = azurerm_storage_account.operator.name
  container_access_type = "private"
}

resource "azurerm_mssql_server" "operator" {
  version             = "12.0"
  name                = "${local.common_resource_name}sv1${var.env}"
  resource_group_name = var.resource_group_name
  location            = var.default_location
  identity {
    type = "SystemAssigned"
  }
  azuread_administrator {
    azuread_authentication_only = true
    login_username              = data.azuread_service_principal.admin.display_name
    object_id                   = data.azuread_service_principal.admin.object_id
    tenant_id                   = data.azurerm_client_config.current.tenant_id
  }
  tags = {
    managedAt = local.managedAt
  }
}

resource "azurerm_role_assignment" "admin_2_rg" {
  principal_id         = data.azuread_service_principal.admin.object_id
  role_definition_name = "Owner"
  scope                = data.azurerm_resource_group.operator.id
}

resource "azurerm_role_assignment" "operator_agent_2_rg" {
  principal_id         = azuread_service_principal.operator_agent.object_id
  role_definition_name = "Owner"
  scope                = data.azurerm_resource_group.operator.id
}

resource "azurerm_role_assignment" "operator_agent_2_storage_account" {
  principal_id         = azuread_service_principal.operator_agent.object_id
  role_definition_name = "Storage Blob Data Owner"
  scope                = azurerm_storage_account.operator.id
}

resource "azurerm_kubernetes_cluster" "operator" {
  name                = "${local.common_resource_name}aks1${var.env}"
  location            = var.default_location
  resource_group_name = var.resource_group_name
  dns_prefix          = "${local.common_resource_name}aks1${var.env}"
  sku_tier            = "Free"

  network_profile {
    load_balancer_sku = "basic"
    network_plugin    = "kubenet"
  }

  default_node_pool {
    name                        = "pool1"
    node_count                  = 1
    vm_size                     = "Standard_B2ms"
    os_disk_size_gb             = "30"
    temporary_name_for_rotation = "tmppool1"
  }

  identity {
    type = "SystemAssigned"
  }

  tags = {
    managedAt = local.managedAt
  }
}