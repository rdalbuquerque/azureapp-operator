## Infra Configuration
The goal here is to use Terraform to provision the foundational infrastructure required to run and validate the operator. This consists of:
- An AKS (Azure Kubernetes Service) cluster, which will host the operator and the respective Kubernetes objects it manages.
- A storage account and container to store each app's state
- SQL Server to house each app's database

### Local Variables

```hcl
locals {
  managedAt            = "github.com/rdalbuquerque/azureapp-operator/infra"
  common_resource_name = "rdak8soperator1"
}
```


### Data Sources

1. **Client Config** (Azure): Fetches Azure client configuration.
2. **Resource Group** (Azure): Targets a resource group by the provided name.
3. **Service Principal** (Azure AD): Targets a service principal with the display name `terraform_agent`.

### Azure AD Application and Service Principal

Creates an Azure AD application named `k8soperator_agent` and an associated service principal.

This will be the operator's identity  in Azure

### Role Assignments

1. **Role Assignment for Operator Agent to Subscription**: Grants the operator agent "Contributor" rights at the subscription level. This allows `k8soperator_agent` to purge key vaults after deletions
2. **Role Assignment for Admin to Resource Group**: Grants the admin "Owner" rights to the targeted resource group.
3. **Role Assignment for Operator Agent to Resource Group**: Grants the operator agent "Owner" rights to the targeted resource group.
4. **Role Assignment for Operator Agent to Storage Account**: Grants the operator agent "Storage Blob Data Owner" rights to the storage account. This allows `k8soperator_agent` to delete blobs from container.

### Azure Resources

1. **Storage Account**:
    - **Name**: Constructed using the local `common_resource_name` and a variable named `env`.
    - **Replication Type**: Locally Redundant Storage (`LRS`).
    - **Tier**: `Standard`.

2. **Storage Container**:
    - **Name**: `state`.
    - **Access**: Private.

3. **SQL Server**:
    - **Version**: `12.0`.
    - **Identity**: System Assigned.
    - **Azure AD Administrator**: Configured using the `admin` service principal.

4. **Kubernetes Cluster (AKS)**:
    - **Network Profile**: Uses basic load balancer and `kubenet` network plugin.
    - **Default Node Pool**: Has a single node of size `Standard_B2ms`.
    - **Identity**: System Assigned.

