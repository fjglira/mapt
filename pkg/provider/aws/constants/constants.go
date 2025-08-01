package constants

const (
	CONFIG_AWS_REGION        string = "aws:region"
	CONFIG_AWS_NATIVE_REGION string = "aws-native:region"
	CONFIG_AWS_ACCESS_KEY    string = "aws:accessKey"
	CONFIG_AWS_SECRET_KEY    string = "aws:secretKey"
)

// LocalStack specific configuration
const (
	CONFIG_AWS_SKIP_CREDENTIALS_VALIDATION string = "aws:skipCredentialsValidation"
	CONFIG_AWS_SKIP_REGION_VALIDATION      string = "aws:skipRegionValidation"
	CONFIG_AWS_SKIP_METADATA_API_CHECK     string = "aws:skipMetadataApiCheck"
	CONFIG_AWS_SKIP_REQUESTING_ACCOUNT_ID  string = "aws:skipRequestingAccountId"
	CONFIG_AWS_S3_USE_PATH_STYLE           string = "aws:s3UsePathStyle"
	CONFIG_AWS_ENDPOINTS                   string = "aws:endpoints"
)

const (
	MetadataBaseURL              = "http://169.254.170.2"
	ECSCredentialsRelativeURIENV = "AWS_CONTAINER_CREDENTIALS_RELATIVE_URI"
	DefaultAWSRegion             = "us-east-1"
	LocalStackEndpoint           = "http://localhost:4566"
)

const (
	PulumiAwsResourceInstance            = "instance"
	PulumiAwsResourceVolume              = "volume"
	PulumiAwsResourceNetworkInterface    = "network-interface"
	PulumiAwsResourceSpotInstanceRequest = "spot-instances-request"
)
