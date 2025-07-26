package aws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/jvkec/aws-s3-simple-sync/internal/fileutils"
)

// bucketexists checks if a bucket exists and is accessible
func (c *Client) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	_, err := c.S3.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})

	if err != nil {
		// check if error is "not found" vs other errors
		var noSuchBucket *types.NoSuchBucket
		if errors.As(err, &noSuchBucket) {
			return false, nil
		}
		return false, fmt.Errorf("error checking bucket: %w", err)
	}

	return true, nil
}

// createbucket creates a new s3 bucket with appropriate settings
func (c *Client) CreateBucket(ctx context.Context, bucketName string) error {
	// check if bucket already exists
	exists, err := c.BucketExists(ctx, bucketName)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("bucket %s already exists", bucketName)
	}

	// create bucket input
	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}

	// for regions other than us-east-1, need to specify location constraint
	if c.Region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(c.Region),
		}
	}

	// create the bucket
	_, err = c.S3.CreateBucket(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
	}

	// enable versioning
	_, err = c.S3.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(bucketName),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to enable versioning on bucket %s: %w", bucketName, err)
	}

	// enable server-side encryption
	_, err = c.S3.PutBucketEncryption(ctx, &s3.PutBucketEncryptionInput{
		Bucket: aws.String(bucketName),
		ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
			Rules: []types.ServerSideEncryptionRule{
				{
					ApplyServerSideEncryptionByDefault: &types.ServerSideEncryptionByDefault{
						SSEAlgorithm: types.ServerSideEncryptionAes256,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to enable encryption on bucket %s: %w", bucketName, err)
	}

	return nil
}

// uploadfile uploads a single file to s3
func (c *Client) UploadFile(ctx context.Context, localPath, bucketName, s3Key string) error {
	// open local file
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", localPath, err)
	}
	defer file.Close()

	// get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// upload file
	_, err = c.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucketName),
		Key:           aws.String(s3Key),
		Body:          file,
		ContentLength: aws.Int64(fileInfo.Size()),
	})

	if err != nil {
		return fmt.Errorf("failed to upload file to s3: %w", err)
	}

	return nil
}

// downloadfile downloads a single file from s3
func (c *Client) DownloadFile(ctx context.Context, bucketName, s3Key, localPath string) error {
	// ensure local directory exists
	localDir := filepath.Dir(localPath)
	if err := fileutils.CreateDirIfNotExists(localDir); err != nil {
		return fmt.Errorf("failed to create local directory: %w", err)
	}

	// get object from s3
	result, err := c.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return fmt.Errorf("failed to download file from s3: %w", err)
	}
	defer result.Body.Close()

	// create local file
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %w", localPath, err)
	}
	defer file.Close()

	// copy data
	_, err = io.Copy(file, result.Body)
	if err != nil {
		return fmt.Errorf("failed to write file data: %w", err)
	}

	return nil
}

// listobjects lists objects in a bucket with a given prefix
func (c *Client) ListObjects(ctx context.Context, bucketName, prefix string) ([]fileutils.FileInfo, error) {
	var files []fileutils.FileInfo
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(bucketName),
		}

		if prefix != "" {
			input.Prefix = aws.String(prefix)
		}

		if continuationToken != nil {
			input.ContinuationToken = continuationToken
		}

		result, err := c.S3.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		// process objects
		for _, obj := range result.Contents {
			if obj.Key == nil {
				continue
			}

			// skip directories (keys ending with /)
			if strings.HasSuffix(*obj.Key, "/") {
				continue
			}

			// get relative path by removing prefix
			relativePath := *obj.Key
			if prefix != "" && strings.HasPrefix(relativePath, prefix) {
				relativePath = strings.TrimPrefix(relativePath, prefix)
				relativePath = strings.TrimPrefix(relativePath, "/")
			}

			// get object metadata for checksum
			etag := ""
			if obj.ETag != nil {
				etag = strings.Trim(*obj.ETag, "\"")
			}

			fileInfo := fileutils.FileInfo{
				Path:         *obj.Key,
				Size:         aws.ToInt64(obj.Size),
				ModTime:      aws.ToTime(obj.LastModified),
				Checksum:     etag, // use etag as checksum for s3 objects
				RelativePath: relativePath,
			}

			files = append(files, fileInfo)
		}

		// check if there are more objects
		if !aws.ToBool(result.IsTruncated) {
			break
		}

		continuationToken = result.NextContinuationToken
	}

	return files, nil
}

// deleteobject deletes an object from s3
func (c *Client) DeleteObject(ctx context.Context, bucketName, s3Key string) error {
	_, err := c.S3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(s3Key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete object %s: %w", s3Key, err)
	}

	return nil
}

// objectexists checks if an object exists in s3
func (c *Client) ObjectExists(ctx context.Context, bucketName, s3Key string) (bool, error) {
	_, err := c.S3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(s3Key),
	})

	if err != nil {
		// check if error is "not found" vs other errors
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return false, nil
		}
		return false, fmt.Errorf("error checking object: %w", err)
	}

	return true, nil
}
