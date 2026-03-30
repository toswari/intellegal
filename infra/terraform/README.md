# Azure Terraform Skeleton

This directory contains a minimal Terraform skeleton for deploying core Azure infrastructure for the Legal Document Intelligence platform.

## What It Creates

- Resource Group
- Log Analytics Workspace
- Container Apps Environment
- Storage Account + Blob Container (for document storage)

## Quick Start

```bash
cd infra/terraform
terraform init
terraform plan \
  -var="project_name=legal-doc-intel" \
  -var="environment=dev" \
  -var="location=westeurope"
```

## Notes

- This is a scaffold, not a full production setup.
- Extend it with Container Apps, PostgreSQL, Key Vault, and networking as implementation matures.
