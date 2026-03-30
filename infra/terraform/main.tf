locals {
  suffix     = lower(regexreplace("${var.project_name}-${var.environment}", "[^a-zA-Z0-9-]", ""))
  base_tags  = merge({ project = var.project_name, environment = var.environment }, var.tags)
  sa_suffix  = substr(lower(regexreplace("${var.project_name}${var.environment}", "[^a-zA-Z0-9]", "")), 0, 14)
}

resource "random_string" "storage" {
  length  = 6
  upper   = false
  lower   = true
  numeric = true
  special = false
}

resource "azurerm_resource_group" "main" {
  name     = "rg-${local.suffix}"
  location = var.location
  tags     = local.base_tags
}

resource "azurerm_log_analytics_workspace" "main" {
  name                = "law-${local.suffix}"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  sku                 = "PerGB2018"
  retention_in_days   = 30
  tags                = local.base_tags
}

resource "azurerm_container_app_environment" "main" {
  name                       = "cae-${local.suffix}"
  location                   = azurerm_resource_group.main.location
  resource_group_name        = azurerm_resource_group.main.name
  log_analytics_workspace_id = azurerm_log_analytics_workspace.main.id
  tags                       = local.base_tags
}

resource "azurerm_storage_account" "documents" {
  name                     = "st${local.sa_suffix}${random_string.storage.result}"
  resource_group_name      = azurerm_resource_group.main.name
  location                 = azurerm_resource_group.main.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  min_tls_version          = "TLS1_2"
  tags                     = local.base_tags
}

resource "azurerm_storage_container" "documents" {
  name                  = "documents"
  storage_account_id    = azurerm_storage_account.documents.id
  container_access_type = "private"
}
