package helpers

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type LocalStackManager struct {
	ContainerName       string
	OriginalAWSEndpoint string
	OriginalEnvVars     map[string]string
}

func NewLocalStackManager() *LocalStackManager {
	// Use random number + timestamp for unique container names
	rand.Seed(time.Now().UnixNano())
	randomSuffix := rand.Intn(100000)
	return &LocalStackManager{
		ContainerName:   fmt.Sprintf("localstack-test-%d-%d", time.Now().Unix(), randomSuffix),
		OriginalEnvVars: make(map[string]string),
	}
}

func (lsm *LocalStackManager) Start() {
	By("Starting LocalStack container")

	// Check if Docker is available
	_, err := exec.LookPath("docker")
	if err != nil {
		Skip("Docker not available, skipping integration tests")
	}

	// Install Pulumi CLI if not available (required by Pulumi Automation API)
	lsm.ensurePulumiCLI()

	// Force cleanup any existing containers with similar names
	lsm.cleanupExistingContainers()

	// Start LocalStack in a docker container
	cmd := exec.Command("docker", "run", "-d", "--name", lsm.ContainerName,
		"-p", "4566:4566",
		"-e", "SERVICES=ec2,iam,s3,sts",
		"-e", "DEBUG=1",
		"-e", "LOCALSTACK_HOST=localhost.localstack.cloud:4566",
		"localstack/localstack:latest")

	err = cmd.Run()
	if err != nil {
		// If container creation fails, try to clean up and retry once
		By(fmt.Sprintf("Initial container creation failed: %v, attempting cleanup and retry", err))
		lsm.cleanupExistingContainers()

		// Generate a new unique name and try again
		rand.Seed(time.Now().UnixNano())
		randomSuffix := rand.Intn(100000)
		lsm.ContainerName = fmt.Sprintf("localstack-test-%d-%d", time.Now().Unix(), randomSuffix)

		cmd = exec.Command("docker", "run", "-d", "--name", lsm.ContainerName,
			"-p", "4566:4566",
			"-e", "SERVICES=ec2,iam,s3,sts",
			"-e", "DEBUG=1",
			"-e", "LOCALSTACK_HOST=localhost.localstack.cloud:4566",
			"localstack/localstack:latest")

		err = cmd.Run()
		Expect(err).NotTo(HaveOccurred())
	}

	// Wait for LocalStack to be ready
	By("Waiting for LocalStack to be ready")
	Eventually(func() error {
		cmd := exec.Command("curl", "-f", "http://localhost:4566/_localstack/health")
		return cmd.Run()
	}, 2*time.Minute, 5*time.Second).Should(Succeed())

	// Configure environment for LocalStack and Pulumi
	lsm.configureEnvironmentForLocalStack()
}

func (lsm *LocalStackManager) Stop() {
	By("Cleaning up LocalStack container")
	cmd := exec.Command("docker", "rm", "-f", lsm.ContainerName)
	cmd.Run() // Ignore errors in cleanup

	// Restore original environment variables
	lsm.restoreEnvironmentVariables()
}

func (lsm *LocalStackManager) cleanupExistingContainers() {
	By("Cleaning up any existing LocalStack containers")

	// List all containers with localstack-test prefix
	cmd := exec.Command("docker", "ps", "-a", "--filter", "name=localstack-test", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return // If we can't list containers, continue anyway
	}

	containerNames := string(output)
	if containerNames != "" {
		// Stop and remove any existing test containers
		stopCmd := exec.Command("docker", "rm", "-f")
		for _, line := range []string{containerNames} {
			if line != "" {
				stopCmd.Args = append(stopCmd.Args, line)
			}
		}
		if len(stopCmd.Args) > 2 { // Only run if we have container names
			stopCmd.Run() // Ignore errors
		}
	}
}

func (lsm *LocalStackManager) configureEnvironmentForLocalStack() {
	By("Configuring environment for LocalStack and Pulumi Go SDK")

	// Environment variables that need to be set for LocalStack
	envVars := map[string]string{
		"AWS_ENDPOINT_URL":         "http://localhost:4566",
		"AWS_ACCESS_KEY_ID":        "test",
		"AWS_SECRET_ACCESS_KEY":    "test",
		"AWS_DEFAULT_REGION":       "us-east-1",
		"AWS_REGION":               "us-east-1",
		"USE_LOCALSTACK":           "true",
		"PULUMI_CONFIG_PASSPHRASE": "test",
	}

	// Save original values and set new ones
	for key, value := range envVars {
		lsm.OriginalEnvVars[key] = os.Getenv(key)
		err := os.Setenv(key, value)
		Expect(err).NotTo(HaveOccurred())
	}
}

func (lsm *LocalStackManager) restoreEnvironmentVariables() {
	By("Restoring original environment variables")

	for key, originalValue := range lsm.OriginalEnvVars {
		if originalValue != "" {
			os.Setenv(key, originalValue)
		} else {
			os.Unsetenv(key)
		}
	}
}

// ensurePulumiCLI installs Pulumi CLI if not available
// This is required by Pulumi's Automation API even when using the Go SDK
func (lsm *LocalStackManager) ensurePulumiCLI() {
	By("Ensuring Pulumi CLI is available")

	// Check if pulumi is already installed
	if _, err := exec.LookPath("pulumi"); err == nil {
		By("Pulumi CLI already available")
		return
	}

	By("Installing Pulumi CLI for integration tests")

	// Create a temporary directory for Pulumi installation
	tmpDir, err := os.MkdirTemp("", "pulumi-install")
	if err != nil {
		Skip(fmt.Sprintf("Failed to create temp directory for Pulumi installation: %v", err))
	}

	// Download and install Pulumi
	installScript := `#!/bin/bash
set -e
curl -fsSL https://get.pulumi.com | sh
export PATH=$HOME/.pulumi/bin:$PATH
pulumi version
`

	scriptPath := fmt.Sprintf("%s/install-pulumi.sh", tmpDir)
	if err := os.WriteFile(scriptPath, []byte(installScript), 0755); err != nil {
		Skip(fmt.Sprintf("Failed to create Pulumi install script: %v", err))
	}

	// Run the installation script
	cmd := exec.Command("bash", scriptPath)
	cmd.Env = append(os.Environ(), "PULUMI_SKIP_UPDATE_CHECK=true")
	if output, err := cmd.CombinedOutput(); err != nil {
		Skip(fmt.Sprintf("Failed to install Pulumi CLI: %v, output: %s", err, output))
	}

	// Add Pulumi to PATH for current process
	home, err := os.UserHomeDir()
	if err != nil {
		Skip(fmt.Sprintf("Failed to get user home directory: %v", err))
	}

	pulumiPath := fmt.Sprintf("%s/.pulumi/bin", home)
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s", pulumiPath, currentPath)
	os.Setenv("PATH", newPath)

	// Verify installation
	if _, err := exec.LookPath("pulumi"); err != nil {
		Skip(fmt.Sprintf("Pulumi CLI not found after installation: %v", err))
	}

	By("Pulumi CLI successfully installed and configured")

	// Clean up temp directory
	os.RemoveAll(tmpDir)
}
