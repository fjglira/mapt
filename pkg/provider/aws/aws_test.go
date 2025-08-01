package aws

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	awsConstants "github.com/redhat-developer/mapt/pkg/provider/aws/constants"
)

// stackConfigSetter is an interface that matches what we need for configuration
type stackConfigSetter interface {
	SetConfig(ctx context.Context, key string, value auto.ConfigValue) error
}

// mockStack implements stackConfigSetter for testing
type mockStack struct {
	configs map[string]string
}

func (m *mockStack) SetConfig(ctx context.Context, key string, value auto.ConfigValue) error {
	if m.configs == nil {
		m.configs = make(map[string]string)
	}
	m.configs[key] = value.Value
	return nil
}

// configureAWSCredentials is the testable version of SetAWSCredentials that accepts an interface
func configureAWSCredentials(ctx context.Context, stack stackConfigSetter, customCredentials map[string]string) error {
	// Configure credentials
	for configKey, envKey := range envCredentials {
		if value, ok := customCredentials[configKey]; ok {
			if err := stack.SetConfig(ctx, configKey,
				auto.ConfigValue{Value: value}); err != nil {
				return err
			}
		} else {
			if err := stack.SetConfig(ctx, configKey,
				auto.ConfigValue{Value: os.Getenv(envKey)}); err != nil {
				return err
			}
		}
	}

	// Configure LocalStack-specific settings if in LocalStack environment
	if isLocalStackEnvironment() {
		// Set LocalStack-specific provider settings
		localStackSettings := map[string]string{
			awsConstants.CONFIG_AWS_SKIP_CREDENTIALS_VALIDATION: "true",
			awsConstants.CONFIG_AWS_SKIP_REGION_VALIDATION:      "true",
			awsConstants.CONFIG_AWS_SKIP_METADATA_API_CHECK:     "true",
			awsConstants.CONFIG_AWS_SKIP_REQUESTING_ACCOUNT_ID:  "true",
			awsConstants.CONFIG_AWS_S3_USE_PATH_STYLE:           "true",
		}

		for configKey, value := range localStackSettings {
			if err := stack.SetConfig(ctx, configKey, auto.ConfigValue{Value: value}); err != nil {
				return err
			}
		}

		// Configure endpoints for LocalStack
		endpoints := configureLocalStackEndpoints()
		endpointsJSON, err := json.Marshal(endpoints)
		if err != nil {
			return err
		}

		if err := stack.SetConfig(ctx, awsConstants.CONFIG_AWS_ENDPOINTS,
			auto.ConfigValue{Value: string(endpointsJSON)}); err != nil {
			return err
		}
	}

	return nil
}

func TestIsLocalStackEnvironment(t *testing.T) {
	tests := []struct {
		name           string
		envVars        map[string]string
		expectedResult bool
	}{
		{
			name: "LocalStack detected via AWS_ENDPOINT_URL with localhost:4566",
			envVars: map[string]string{
				"AWS_ENDPOINT_URL": "http://localhost:4566",
			},
			expectedResult: true,
		},
		{
			name: "LocalStack detected via AWS_ENDPOINT_URL with localstack hostname",
			envVars: map[string]string{
				"AWS_ENDPOINT_URL": "http://localstack:4566",
			},
			expectedResult: true,
		},
		{
			name: "LocalStack detected via LOCALSTACK_HOSTNAME",
			envVars: map[string]string{
				"LOCALSTACK_HOSTNAME": "localhost",
			},
			expectedResult: true,
		},
		{
			name: "LocalStack detected via USE_LOCALSTACK flag",
			envVars: map[string]string{
				"USE_LOCALSTACK": "true",
			},
			expectedResult: true,
		},
		{
			name: "Regular AWS environment (no LocalStack indicators)",
			envVars: map[string]string{
				"AWS_REGION": "us-east-1",
			},
			expectedResult: false,
		},
		{
			name: "Real AWS endpoint should not trigger LocalStack detection",
			envVars: map[string]string{
				"AWS_ENDPOINT_URL": "https://ec2.us-east-1.amazonaws.com",
			},
			expectedResult: false,
		},
		{
			name:           "Empty environment should not detect LocalStack",
			envVars:        map[string]string{},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original environment
			originalEnv := make(map[string]string)
			for key := range tt.envVars {
				originalEnv[key] = os.Getenv(key)
			}

			// Clear relevant environment variables first
			clearEnvVars := []string{"AWS_ENDPOINT_URL", "LOCALSTACK_HOSTNAME", "USE_LOCALSTACK"}
			for _, key := range clearEnvVars {
				os.Unsetenv(key)
			}

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Test the function
			result := isLocalStackEnvironment()
			if result != tt.expectedResult {
				t.Errorf("isLocalStackEnvironment() = %v, expected %v", result, tt.expectedResult)
			}

			// Restore original environment
			for key, value := range originalEnv {
				if value == "" {
					os.Unsetenv(key)
				} else {
					os.Setenv(key, value)
				}
			}
		})
	}
}

func TestConfigureLocalStackEndpoints(t *testing.T) {
	tests := []struct {
		name                 string
		endpointURL          string
		expectedEndpoint     string
		expectedServiceCount int
	}{
		{
			name:                 "Default LocalStack endpoint when AWS_ENDPOINT_URL not set",
			endpointURL:          "",
			expectedEndpoint:     awsConstants.LocalStackEndpoint,
			expectedServiceCount: 27, // Number of services in the list
		},
		{
			name:                 "Custom LocalStack endpoint from environment",
			endpointURL:          "http://my-localstack:4566",
			expectedEndpoint:     "http://my-localstack:4566",
			expectedServiceCount: 27,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and set environment
			originalEndpoint := os.Getenv("AWS_ENDPOINT_URL")
			if tt.endpointURL == "" {
				os.Unsetenv("AWS_ENDPOINT_URL")
			} else {
				os.Setenv("AWS_ENDPOINT_URL", tt.endpointURL)
			}

			// Test the function
			endpoints := configureLocalStackEndpoints()

			// Verify the number of endpoints
			if len(endpoints) != tt.expectedServiceCount {
				t.Errorf("configureLocalStackEndpoints() returned %d endpoints, expected %d",
					len(endpoints), tt.expectedServiceCount)
			}

			// Verify each endpoint contains the expected URL
			for i, endpoint := range endpoints {
				if len(endpoint) != 1 {
					t.Errorf("Endpoint %d should have exactly 1 key-value pair, got %d", i, len(endpoint))
					continue
				}

				for service, url := range endpoint {
					if url != tt.expectedEndpoint {
						t.Errorf("Service %s has endpoint %v, expected %s", service, url, tt.expectedEndpoint)
					}
				}
			}

			// Verify specific services are included
			expectedServices := []string{"ec2", "s3", "iam", "lambda", "sts"}
			foundServices := make(map[string]bool)
			for _, endpoint := range endpoints {
				for service := range endpoint {
					foundServices[service] = true
				}
			}

			for _, service := range expectedServices {
				if !foundServices[service] {
					t.Errorf("Expected service %s not found in endpoints", service)
				}
			}

			// Restore environment
			if originalEndpoint == "" {
				os.Unsetenv("AWS_ENDPOINT_URL")
			} else {
				os.Setenv("AWS_ENDPOINT_URL", originalEndpoint)
			}
		})
	}
}

func TestSetAWSCredentials_RegularAWS(t *testing.T) {
	// Setup test environment for regular AWS (no LocalStack)
	originalEnvVars := map[string]string{
		"AWS_ENDPOINT_URL":      os.Getenv("AWS_ENDPOINT_URL"),
		"AWS_ACCESS_KEY_ID":     os.Getenv("AWS_ACCESS_KEY_ID"),
		"AWS_SECRET_ACCESS_KEY": os.Getenv("AWS_SECRET_ACCESS_KEY"),
		"AWS_DEFAULT_REGION":    os.Getenv("AWS_DEFAULT_REGION"),
		"USE_LOCALSTACK":        os.Getenv("USE_LOCALSTACK"),
		"LOCALSTACK_HOSTNAME":   os.Getenv("LOCALSTACK_HOSTNAME"),
	}

	// Set up regular AWS environment
	os.Unsetenv("AWS_ENDPOINT_URL")
	os.Unsetenv("USE_LOCALSTACK")
	os.Unsetenv("LOCALSTACK_HOSTNAME")
	os.Setenv("AWS_ACCESS_KEY_ID", "AKIATEST123")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "secret123")
	os.Setenv("AWS_DEFAULT_REGION", "us-west-2")

	// Create mock stack
	mockStack := &mockStack{}
	ctx := context.Background()

	// Test configureAWSCredentials
	err := configureAWSCredentials(ctx, mockStack, nil)
	if err != nil {
		t.Fatalf("configureAWSCredentials() failed: %v", err)
	}

	// Verify only basic credentials are set (no LocalStack config)
	expectedConfigs := map[string]string{
		awsConstants.CONFIG_AWS_REGION:        "us-west-2",
		awsConstants.CONFIG_AWS_NATIVE_REGION: "us-west-2",
		awsConstants.CONFIG_AWS_ACCESS_KEY:    "AKIATEST123",
		awsConstants.CONFIG_AWS_SECRET_KEY:    "secret123",
	}

	for key, expectedValue := range expectedConfigs {
		if actualValue, exists := mockStack.configs[key]; !exists {
			t.Errorf("Expected config %s to be set", key)
		} else if actualValue != expectedValue {
			t.Errorf("Config %s = %s, expected %s", key, actualValue, expectedValue)
		}
	}

	// Verify LocalStack-specific configs are NOT set
	localStackConfigs := []string{
		awsConstants.CONFIG_AWS_SKIP_CREDENTIALS_VALIDATION,
		awsConstants.CONFIG_AWS_SKIP_REGION_VALIDATION,
		awsConstants.CONFIG_AWS_SKIP_METADATA_API_CHECK,
		awsConstants.CONFIG_AWS_SKIP_REQUESTING_ACCOUNT_ID,
		awsConstants.CONFIG_AWS_S3_USE_PATH_STYLE,
		awsConstants.CONFIG_AWS_ENDPOINTS,
	}

	for _, config := range localStackConfigs {
		if _, exists := mockStack.configs[config]; exists {
			t.Errorf("LocalStack config %s should not be set in regular AWS mode", config)
		}
	}

	// Restore environment
	for key, value := range originalEnvVars {
		if value == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, value)
		}
	}
}

func TestSetAWSCredentials_LocalStack(t *testing.T) {
	// Setup test environment for LocalStack
	originalEnvVars := map[string]string{
		"AWS_ENDPOINT_URL":      os.Getenv("AWS_ENDPOINT_URL"),
		"AWS_ACCESS_KEY_ID":     os.Getenv("AWS_ACCESS_KEY_ID"),
		"AWS_SECRET_ACCESS_KEY": os.Getenv("AWS_SECRET_ACCESS_KEY"),
		"AWS_DEFAULT_REGION":    os.Getenv("AWS_DEFAULT_REGION"),
		"USE_LOCALSTACK":        os.Getenv("USE_LOCALSTACK"),
	}

	// Set up LocalStack environment
	os.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	os.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	os.Setenv("USE_LOCALSTACK", "true")

	// Create mock stack
	mockStack := &mockStack{}
	ctx := context.Background()

	// Test configureAWSCredentials
	err := configureAWSCredentials(ctx, mockStack, nil)
	if err != nil {
		t.Fatalf("configureAWSCredentials() failed: %v", err)
	}

	// Verify basic credentials are set
	expectedCredentials := map[string]string{
		awsConstants.CONFIG_AWS_REGION:        "us-east-1",
		awsConstants.CONFIG_AWS_NATIVE_REGION: "us-east-1",
		awsConstants.CONFIG_AWS_ACCESS_KEY:    "test",
		awsConstants.CONFIG_AWS_SECRET_KEY:    "test",
	}

	for key, expectedValue := range expectedCredentials {
		if actualValue, exists := mockStack.configs[key]; !exists {
			t.Errorf("Expected credential config %s to be set", key)
		} else if actualValue != expectedValue {
			t.Errorf("Credential config %s = %s, expected %s", key, actualValue, expectedValue)
		}
	}

	// Verify LocalStack-specific configs are set
	expectedLocalStackConfigs := map[string]string{
		awsConstants.CONFIG_AWS_SKIP_CREDENTIALS_VALIDATION: "true",
		awsConstants.CONFIG_AWS_SKIP_REGION_VALIDATION:      "true",
		awsConstants.CONFIG_AWS_SKIP_METADATA_API_CHECK:     "true",
		awsConstants.CONFIG_AWS_SKIP_REQUESTING_ACCOUNT_ID:  "true",
		awsConstants.CONFIG_AWS_S3_USE_PATH_STYLE:           "true",
	}

	for key, expectedValue := range expectedLocalStackConfigs {
		if actualValue, exists := mockStack.configs[key]; !exists {
			t.Errorf("Expected LocalStack config %s to be set", key)
		} else if actualValue != expectedValue {
			t.Errorf("LocalStack config %s = %s, expected %s", key, actualValue, expectedValue)
		}
	}

	// Verify endpoints configuration is set and valid
	if endpointsJSON, exists := mockStack.configs[awsConstants.CONFIG_AWS_ENDPOINTS]; !exists {
		t.Error("Expected AWS endpoints config to be set")
	} else {
		// Parse and verify the endpoints JSON
		var endpoints []map[string]interface{}
		if err := json.Unmarshal([]byte(endpointsJSON), &endpoints); err != nil {
			t.Errorf("Failed to parse endpoints JSON: %v", err)
		} else {
			// Verify endpoints contain expected services and URL
			expectedEndpoint := "http://localhost:4566"
			foundServices := make(map[string]bool)

			for _, endpoint := range endpoints {
				for service, url := range endpoint {
					foundServices[service] = true
					if url != expectedEndpoint {
						t.Errorf("Service %s has endpoint %v, expected %s", service, url, expectedEndpoint)
					}
				}
			}

			// Check for key services
			keyServices := []string{"ec2", "s3", "iam", "sts"}
			for _, service := range keyServices {
				if !foundServices[service] {
					t.Errorf("Expected service %s not found in endpoints configuration", service)
				}
			}
		}
	}

	// Restore environment
	for key, value := range originalEnvVars {
		if value == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, value)
		}
	}
}

func TestSetAWSCredentials_CustomCredentials(t *testing.T) {
	// Test with custom credentials that override environment variables
	originalEnvVars := map[string]string{
		"AWS_ENDPOINT_URL":      os.Getenv("AWS_ENDPOINT_URL"),
		"AWS_ACCESS_KEY_ID":     os.Getenv("AWS_ACCESS_KEY_ID"),
		"AWS_SECRET_ACCESS_KEY": os.Getenv("AWS_SECRET_ACCESS_KEY"),
		"AWS_DEFAULT_REGION":    os.Getenv("AWS_DEFAULT_REGION"),
	}

	// Set environment vars that should be overridden
	os.Setenv("AWS_ACCESS_KEY_ID", "env_access_key")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "env_secret_key")
	os.Setenv("AWS_DEFAULT_REGION", "env-region")
	os.Unsetenv("AWS_ENDPOINT_URL") // Regular AWS mode

	// Create mock stack
	mockStack := &mockStack{}
	ctx := context.Background()

	// Test with custom credentials
	customCredentials := map[string]string{
		awsConstants.CONFIG_AWS_ACCESS_KEY: "custom_access_key",
		awsConstants.CONFIG_AWS_SECRET_KEY: "custom_secret_key",
		awsConstants.CONFIG_AWS_REGION:     "custom-region",
	}

	err := configureAWSCredentials(ctx, mockStack, customCredentials)
	if err != nil {
		t.Fatalf("configureAWSCredentials() failed: %v", err)
	}

	// Verify custom credentials override environment variables
	expectedConfigs := map[string]string{
		awsConstants.CONFIG_AWS_ACCESS_KEY:    "custom_access_key",
		awsConstants.CONFIG_AWS_SECRET_KEY:    "custom_secret_key",
		awsConstants.CONFIG_AWS_REGION:        "custom-region",
		awsConstants.CONFIG_AWS_NATIVE_REGION: "env-region", // This one comes from env since not in custom
	}

	for key, expectedValue := range expectedConfigs {
		if actualValue, exists := mockStack.configs[key]; !exists {
			t.Errorf("Expected config %s to be set", key)
		} else if actualValue != expectedValue {
			t.Errorf("Config %s = %s, expected %s", key, actualValue, expectedValue)
		}
	}

	// Restore environment
	for key, value := range originalEnvVars {
		if value == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, value)
		}
	}
}
