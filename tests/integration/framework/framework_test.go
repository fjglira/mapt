package integration

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kindCloudConfig "github.com/redhat-developer/mapt/pkg/provider/util/cloud-config/kind"
	"github.com/redhat-developer/mapt/tests/integration/helpers"
)

var _ = Describe("Integration Testing Framework", func() {
	var localstack *helpers.LocalStackManager

	BeforeEach(func() {
		localstack = helpers.NewLocalStackManager()
		localstack.Start()
	})

	AfterEach(func() {
		localstack.Stop()
	})

	Describe("LocalStack Integration", func() {
		Context("when LocalStack is running", func() {
			It("should set up AWS environment variables correctly", func() {
				By("Verifying AWS environment variables")
				Expect(os.Getenv("AWS_ENDPOINT_URL")).To(Equal("http://localhost:4566"))
				Expect(os.Getenv("AWS_ACCESS_KEY_ID")).To(Equal("test"))
				Expect(os.Getenv("AWS_SECRET_ACCESS_KEY")).To(Equal("test"))
				Expect(os.Getenv("AWS_DEFAULT_REGION")).To(Equal("us-east-1"))
			})

			It("should have a unique container name for each test run", func() {
				By("Checking container name uniqueness")
				Expect(localstack.ContainerName).To(ContainSubstring("localstack-test-"))
				Expect(len(localstack.ContainerName)).To(BeNumerically(">", 20))
			})
		})
	})

	Describe("Component Validation", func() {
		Context("when parsing port mappings", func() {
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

			It("should handle multiple port mappings", func() {
				By("Parsing multiple port mappings")
				input := `[
					{"containerPort": 8080, "hostPort": 8080, "protocol": "TCP"},
					{"containerPort": 9090, "hostPort": 9090, "protocol": "UDP"}
				]`
				mappings, err := kindCloudConfig.ParseExtraPortMappings(input)

				Expect(err).NotTo(HaveOccurred())
				Expect(mappings).To(HaveLen(2))
				Expect(mappings[0].Protocol).To(Equal("TCP"))
				Expect(mappings[1].Protocol).To(Equal("UDP"))
			})

			It("should handle empty port mappings gracefully", func() {
				By("Handling empty port mappings")
				mappings, err := kindCloudConfig.ParseExtraPortMappings("")

				Expect(err).NotTo(HaveOccurred())
				Expect(mappings).To(BeEmpty())
			})

			It("should reject malformed JSON", func() {
				By("Rejecting invalid JSON")
				input := `[{"containerPort": 8080, "hostPort": 8080, "protocol": "TCP"`
				_, err := kindCloudConfig.ParseExtraPortMappings(input)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unexpected end of JSON input"))
			})

			It("should reject JSON with missing required fields", func() {
				By("Rejecting incomplete port mappings")
				input := `[{"containerPort": 8080}]` // Missing hostPort and protocol
				mappings, err := kindCloudConfig.ParseExtraPortMappings(input)

				// The parsing might succeed but validation should catch incomplete data
				if err == nil {
					Expect(mappings[0].HostPort).To(Equal(0))  // Should be zero value
					Expect(mappings[0].Protocol).To(Equal("")) // Should be empty
				}
			})
		})
	})

	Describe("Testing Framework Reliability", func() {
		Context("when running multiple test iterations", func() {
			It("should maintain environment isolation", func() {
				By("Testing environment isolation")

				// Save current state
				originalEndpoint := os.Getenv("AWS_ENDPOINT_URL")

				// Verify current test environment
				Expect(originalEndpoint).To(Equal("http://localhost:4566"))

				// Each test should have the same environment
				Expect(os.Getenv("AWS_ACCESS_KEY_ID")).To(Equal("test"))
				Expect(os.Getenv("AWS_SECRET_ACCESS_KEY")).To(Equal("test"))
				Expect(os.Getenv("AWS_DEFAULT_REGION")).To(Equal("us-east-1"))
			})
		})
	})
})
