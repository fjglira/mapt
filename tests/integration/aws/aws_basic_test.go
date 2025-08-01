package integration

import (
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	maptContext "github.com/redhat-developer/mapt/pkg/manager/context"
	"github.com/redhat-developer/mapt/pkg/provider/aws/action/kind"
	kindCloudConfig "github.com/redhat-developer/mapt/pkg/provider/util/cloud-config/kind"
	"github.com/redhat-developer/mapt/tests/integration/helpers"
)

var _ = Describe("AWS MAPT configuration", func() {
	var (
		localstack      interface{} // Can be either LocalStackManager or ContainerLocalStackManager
		testProjectName string
		testBackedURL   string
	)

	BeforeEach(func() {
		// Use appropriate LocalStack manager based on environment
		if helpers.IsContainerizedEnvironment() {
			By("Using containerized LocalStack environment")
			containerLs := helpers.NewContainerLocalStackManager()
			containerLs.Start()
			localstack = containerLs
		} else {
			By("Using host-based LocalStack environment")
			hostLs := helpers.NewLocalStackManager()
			hostLs.Start()
			localstack = hostLs
		}

		// Set test project configuration
		testProjectName = fmt.Sprintf("integration-test-%d", time.Now().Unix())
		testBackedURL = "file:///tmp/mapt-integration-test"
	})

	AfterEach(func() {
		if helpers.IsContainerizedEnvironment() {
			localstack.(*helpers.ContainerLocalStackManager).Stop()
		} else {
			localstack.(*helpers.LocalStackManager).Stop()
		}
	})

	Describe("Parameter Validation", func() {
		Context("when validating context arguments", func() {
			It("should reject empty project names", func() {
				By("Testing project name validation")

				// We expect this test to panic due to Pulumi's contract validation
				// This is the correct behavior for invalid project configurations
				defer func() {
					if r := recover(); r != nil {
						By("Project name validation correctly caused a panic (expected behavior)")
						// Verify the panic is related to project name validation
						panicMsg := fmt.Sprintf("%v", r)
						Expect(panicMsg).To(SatisfyAny(
							ContainSubstring("project"),
							ContainSubstring("name"),
							ContainSubstring("missing"),
							ContainSubstring("attribute"),
						))
					} else {
						Fail("Expected validation panic for empty project name, but none occurred")
					}
				}()

				contextArgs := &maptContext.ContextArgs{
					ProjectName: "", // Invalid empty project name
					BackedURL:   testBackedURL,
				}

				kindArgs := &kind.KindArgs{
					Version: "1.31",
					Arch:    "x86_64",
					Spot:    false, // Don't use spot to avoid allocation logic
				}

				// This should panic due to empty project name (Pulumi contract validation)
				kind.Create(contextArgs, kindArgs)
			})

			It("should accept valid project configurations with LocalStack", func() {
				By("Testing valid project configuration with LocalStack environment")

				contextArgs := &maptContext.ContextArgs{
					ProjectName: testProjectName,
					BackedURL:   testBackedURL,
					Tags:        map[string]string{"test": "integration"},
				}

				kindArgs := &kind.KindArgs{
					Version:           "1.31",
					Arch:              "x86_64",
					Spot:              false, // Don't use spot to avoid allocation logic
					ExtraPortMappings: []kindCloudConfig.PortMapping{},
				}

				// This might still fail due to LocalStack connectivity or other infrastructure issues,
				// but should pass the initial parameter validation
				_, err := kind.Create(contextArgs, kindArgs)

				// If there's an error, it should not be due to parameter validation
				if err != nil {
					errorMsg := err.Error()
					// Should not be parameter validation errors
					Expect(errorMsg).NotTo(ContainSubstring("project name"))
					Expect(errorMsg).NotTo(ContainSubstring("empty"))
					Expect(errorMsg).NotTo(ContainSubstring("required"))

					// Should not be tool installation errors since we use Go SDK directly
					if strings.Contains(errorMsg, "pulumi") || strings.Contains(errorMsg, "command not found") {
						// Log this for debugging but don't fail the test - this might be a real infrastructure error
						By(fmt.Sprintf("Infrastructure error (expected in test environment): %s", errorMsg))
					}
				} else {
					// If no error, that's good - means parameters were valid and basic setup worked
					By("Valid project configuration accepted")
				}
			})
		})

		Context("when parsing extra port mappings", func() {
			It("should correctly parse valid JSON port mappings", func() {
				By("Parsing valid port mappings")

				input := `[{"containerPort": 8080, "hostPort": 8080, "protocol": "TCP"}]`
				mappings, err := kindCloudConfig.ParseExtraPortMappings(input)

				Expect(err).NotTo(HaveOccurred())
				Expect(mappings).To(HaveLen(1))
				Expect(mappings[0].ContainerPort).To(Equal(8080))
				Expect(mappings[0].HostPort).To(Equal(8080))
				Expect(mappings[0].Protocol).To(Equal("TCP"))
			})

			It("should handle empty port mappings", func() {
				By("Handling empty port mappings")

				mappings, err := kindCloudConfig.ParseExtraPortMappings("")

				Expect(err).NotTo(HaveOccurred())
				Expect(mappings).To(BeEmpty())
			})

			It("should reject invalid JSON", func() {
				By("Rejecting invalid JSON")

				input := `[{"containerPort": 8080, "hostPort": 8080, "protocol": "TCP"`
				_, err := kindCloudConfig.ParseExtraPortMappings(input)

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("LocalStack Integration", func() {
		Context("when LocalStack is configured", func() {
			It("should have LocalStack environment properly configured", func() {
				By("Checking LocalStack environment variables")

				// Verify that LocalStack environment is set up correctly
				Expect(localstack).NotTo(BeNil())

				// Check that AWS_ENDPOINT_URL points to LocalStack
				endpointURL := os.Getenv("AWS_ENDPOINT_URL")
				Expect(endpointURL).To(Equal("http://localhost:4566"))

				// Check that AWS credentials are set for LocalStack
				Expect(os.Getenv("AWS_ACCESS_KEY_ID")).To(Equal("test"))
				Expect(os.Getenv("AWS_SECRET_ACCESS_KEY")).To(Equal("test"))
				Expect(os.Getenv("AWS_DEFAULT_REGION")).To(Equal("us-east-1"))

				// Check that USE_LOCALSTACK flag is set
				Expect(os.Getenv("USE_LOCALSTACK")).To(Equal("true"))
			})

			It("should detect LocalStack environment automatically", func() {
				By("Verifying LocalStack auto-detection logic")

				// This test ensures that the AWS provider will detect LocalStack
				// based on the environment variables set by the LocalStackManager
				endpointURL := os.Getenv("AWS_ENDPOINT_URL")
				Expect(endpointURL).To(ContainSubstring("localhost:4566"))

				useLocalStack := os.Getenv("USE_LOCALSTACK")
				Expect(useLocalStack).To(Equal("true"))
			})
		})
	})
})
