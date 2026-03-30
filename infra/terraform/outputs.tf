output "resource_group_name" {
  description = "Resource group name."
  value       = azurerm_resource_group.main.name
}

output "container_app_environment_id" {
  description = "Container Apps environment ID."
  value       = azurerm_container_app_environment.main.id
}

output "storage_account_name" {
  description = "Storage account name."
  value       = azurerm_storage_account.documents.name
}

output "storage_container_name" {
  description = "Documents blob container name."
  value       = azurerm_storage_container.documents.name
}
