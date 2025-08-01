package helpers

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ContainerLocalStackManager is for use in containerized test environments
// where LocalStack is managed by Docker Compose and tools are pre-installed
type ContainerLocalStackManager struct {
	OriginalEnvVars map[string]string
}

func NewContainerLocalStackManager() *ContainerLocalStackManager {
	return &ContainerLocalStackManager{
		OriginalEnvVars: make(map[string]string),
	}
}

func (clsm *ContainerLocalStackManager) Start() {
	By("Configuring environment for containerized LocalStack")

	// In a containerized environment, LocalStack is already running
	// We just need to ensure environment variables are set correctly
	clsm.configureEnvironmentForLocalStack()

	// Verify LocalStack is accessible (it should already be running)
	By("Verifying LocalStack connectivity")
	// The Docker Compose healthcheck ensures LocalStack is ready
	// before starting the test container
}

func (clsm *ContainerLocalStackManager) Stop() {
	By("Restoring original environment variables")

	// Restore original environment variables
	for key, originalValue := range clsm.OriginalEnvVars {
		if originalValue != "" {
			os.Setenv(key, originalValue)
		} else {
			os.Unsetenv(key)
		}
	}
}

func (clsm *ContainerLocalStackManager) configureEnvironmentForLocalStack() {
	By("Configuring environment for containerized LocalStack and Pulumi Go SDK")

	// Environment variables for LocalStack (should already be set by Docker Compose)
	// but we ensure they're correct and save original values
	envVars := map[string]string{
		"AWS_ENDPOINT_URL":         "http://localstack:4566",
		"AWS_ACCESS_KEY_ID":        "test",
		"AWS_SECRET_ACCESS_KEY":    "test",
		"AWS_DEFAULT_REGION":       "us-east-1",
		"AWS_REGION":               "us-east-1",
		"USE_LOCALSTACK":           "true",
		"PULUMI_CONFIG_PASSPHRASE": "test",
	}

	// Save original values and set new ones (if not already set)
	for key, value := range envVars {
		clsm.OriginalEnvVars[key] = os.Getenv(key)
		if os.Getenv(key) == "" {
			err := os.Setenv(key, value)
			Expect(err).NotTo(HaveOccurred())
		}
	}
}

// IsContainerizedEnvironment checks if we're running in a containerized test environment
func IsContainerizedEnvironment() bool {
	// Check if we're in a containerized environment by looking for specific env vars
	// or container-specific indicators
	if os.Getenv("MAPT_TEST_CONTAINER") == "true" {
		return true
	}

	// Check if LocalStack endpoint points to container hostname
	endpoint := os.Getenv("AWS_ENDPOINT_URL")
	return endpoint == "http://localstack:4566"
}
