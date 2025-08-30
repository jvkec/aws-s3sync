package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	appConfig "github.com/jvkec/aws-s3sync/internal/config"
)

// client wraps the aws s3 client with configuration
type Client struct {
	S3     *s3.Client
	Config *appConfig.Config
	Region string
}

// newclient creates a new aws client from the application configuration
func NewClient(appConfig *appConfig.Config) (*Client, error) {
	var cfg aws.Config
	var err error

	ctx := context.Background()

	// load aws configuration based on app config
	if appConfig.AWS.Profile != "" {
		// use aws profile
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(appConfig.AWS.Region),
			config.WithSharedConfigProfile(appConfig.AWS.Profile),
		)
	} else if appConfig.AWS.AccessKeyID != "" && appConfig.AWS.SecretAccessKey != "" {
		// use explicit credentials
		creds := credentials.NewStaticCredentialsProvider(
			appConfig.AWS.AccessKeyID,
			appConfig.AWS.SecretAccessKey,
			appConfig.AWS.SessionToken,
		)

		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(appConfig.AWS.Region),
			config.WithCredentialsProvider(creds),
		)
	} else {
		// use default aws credential chain
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(appConfig.AWS.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	// create s3 client
	s3Client := s3.NewFromConfig(cfg)

	return &Client{
		S3:     s3Client,
		Config: appConfig,
		Region: appConfig.AWS.Region,
	}, nil
}

// testconnection verifies that the aws credentials are working
func (c *Client) TestConnection(ctx context.Context) error {
	// test connection by listing buckets
	_, err := c.S3.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("failed to connect to aws s3: %w", err)
	}

	return nil
}

// listbuckets returns a list of accessible s3 buckets
func (c *Client) ListBuckets(ctx context.Context) ([]string, error) {
	result, err := c.S3.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	buckets := make([]string, 0, len(result.Buckets))
	for _, bucket := range result.Buckets {
		if bucket.Name != nil {
			buckets = append(buckets, *bucket.Name)
		}
	}

	return buckets, nil
}
