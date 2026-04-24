package platform

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func NewMinio(endpoint, accessKey, secretKey string, useSSL bool) (*minio.Client, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}
	return client, nil
}

func EnsureBucket(client *minio.Client, bucketName string) error {
	exists, err := client.BucketExists(context.Background(), bucketName)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		err = client.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("make bucket: %w", err)
		}
	}
	return nil
}
