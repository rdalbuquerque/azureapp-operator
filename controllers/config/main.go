package config

import (
	"fmt"
	"os"
)

type Config struct {
	TerraformBasePath       string
	TerraformExecutablePath string
	ARMTenantID             string
	ARMSubscriptionID       string
	ARMClientID             string
	ARMClientSecret         string
	ResourceGroup           string
	StorageAccount          string
	Container               string
}

func LoadConfig() Config {
	return Config{
		TerraformBasePath:       getRequiredEnv("TF_BASE_PATH"),
		TerraformExecutablePath: getRequiredEnv("TF_EXECUTABLE_PATH"),
		ARMTenantID:             getRequiredEnv("ARM_TENANT_ID"),
		ARMSubscriptionID:       getRequiredEnv("ARM_SUBSCRIPTION_ID"),
		ARMClientID:             getRequiredEnv("ARM_CLIENT_ID"),
		ARMClientSecret:         getRequiredEnv("ARM_CLIENT_SECRET"),
		ResourceGroup:           getRequiredEnv("TF_BACKEND_RESOURCE_GROUP"),
		StorageAccount:          getRequiredEnv("TF_BACKEND_STORAGE_ACCOUNT"),
		Container:               getEnv("TF_BACKEND_CONTAINER", "state"),
	}
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
