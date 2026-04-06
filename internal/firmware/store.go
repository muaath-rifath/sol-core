package firmware

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

type Store struct {
	client *minio.Client
	bucket string
}

func NewStore(client *minio.Client, bucket string) *Store {
	return &Store{client: client, bucket: bucket}
}

func (s *Store) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
	}
	return nil
}

func (s *Store) Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("upload firmware: %w", err)
	}
	return nil
}

func (s *Store) Download(ctx context.Context, objectName string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("download firmware: %w", err)
	}
	return obj, nil
}

func (s *Store) List(ctx context.Context) ([]minio.ObjectInfo, error) {
	var objects []minio.ObjectInfo
	for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("list firmware: %w", obj.Err)
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

func (s *Store) Delete(ctx context.Context, objectName string) error {
	return s.client.RemoveObject(ctx, s.bucket, objectName, minio.RemoveObjectOptions{})
}
