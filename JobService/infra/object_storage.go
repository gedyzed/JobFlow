package infra

import (
	"context"
	"io"

	"github.com/cockroachdb/errors"
	"github.com/gedyzed/JobFlow/JobService/infra/configs"
	"github.com/gedyzed/JobFlow/JobService/models"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ObjectStorage struct {
	config configs.ObjectStorageConfig
	client *s3.Client
}

func NewObjectStorage(ctx context.Context, cfg configs.ObjectStorageConfig)(models.IObjectStorage, error) {

	awsConfig, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID, 
				cfg.SecretAccessKey, 
				"",
				),
			),
		)
		
	if err != nil {
		return nil, errors.Wrap(err, "failed to load AWS configuration")
	}

	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	})

	return &ObjectStorage{
		config: cfg,
		client: client,
	}, nil
}

func (os *ObjectStorage) GetObject(ctx context.Context, objectKey string) (io.Reader, error) {

	out, err := os.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(os.config.BucketName),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to get object from S3")
	}
	return out.Body, nil
}

func (os *ObjectStorage) PutObject(ctx context.Context, objectKey string, data io.Reader, contentType *string) error {
	 _, err := os.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(os.config.BucketName),
		Key:    aws.String(objectKey), 
		Body:  data,
		ContentType: aws.String(*contentType),
	})

	return err
}

func (os *ObjectStorage) DeleteObject(ctx context.Context, objectKey string) error {
	_, err := os.client.DeleteObject(ctx, &s3.DeleteObjectInput{		
		Bucket: aws.String(os.config.BucketName),
		Key:    aws.String(objectKey),
	})

	return err 
}