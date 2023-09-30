package config

import (
	"fmt"
	"os"
)

type ConfigOptions struct {
	TerraformBasePath              string
	TerraformExecutablePath        string
	TerraformBackendResourceGroup  string
	TerraformBackendStorageAccount string
	TerraformBackendContainer      string
	ARMTenantID                    string
	ARMSubscriptionID              string
	ARMClientID                    string
	ARMClientSecret                string
	ResourceGroup                  string
	StorageAccount                 string
	Container                      string
	DefaultSQLServer               string
}

var Config = &ConfigOptions{}

func SetConfig() {
	Config.TerraformBasePath = getRequiredEnv("TF_BASE_PATH")
	Config.TerraformExecutablePath = getRequiredEnv("TF_EXECUTABLE_PATH")
	Config.ARMTenantID = getRequiredEnv("ARM_TENANT_ID")
	Config.ARMSubscriptionID = getRequiredEnv("ARM_SUBSCRIPTION_ID")
	Config.ARMClientID = getRequiredEnv("ARM_CLIENT_ID")
	Config.ARMClientSecret = getRequiredEnv("ARM_CLIENT_SECRET")
	Config.TerraformBackendResourceGroup = getRequiredEnv("TF_BACKEND_RESOURCE_GROUP")
	Config.TerraformBackendStorageAccount = getRequiredEnv("TF_BACKEND_STORAGE_ACCOUNT")
	Config.TerraformBackendContainer = getEnv("TF_BACKEND_CONTAINER", "state")
	Config.DefaultSQLServer = getRequiredEnv("DEFAULT_SQL_SERVER")
}

func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

func getRequiredEnv(key string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	panic(fmt.Sprintf("Environment variable %s is required but not set", key))
}
