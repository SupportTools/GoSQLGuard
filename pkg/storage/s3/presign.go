package s3

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// GeneratePresignedURL creates a presigned URL for downloading an S3 object
func (c *Client) GeneratePresignedURL(objectKey string, expiryTime time.Duration) (string, error) {
	// Create a presigner from the S3 client
	presignClient := s3.NewPresignClient(c.s3Client)

	// Create a GetObject request
	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(c.cfg.S3.Bucket),
		Key:    aws.String(objectKey),
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1440*time.Second)
	defer cancel()

	// Generate the presigned URL
	presignResult, err := presignClient.PresignGetObject(ctx, getObjectInput, func(opts *s3.PresignOptions) {
		opts.Expires = expiryTime
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	log.Printf("Generated presigned URL for S3 object %s (expires in %s)", objectKey, expiryTime)
	return presignResult.URL, nil
}
