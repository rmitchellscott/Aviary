package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/rmitchellscott/aviary/internal/logging"
)

type S3Backend struct {
	client   *s3.Client
	uploader *manager.Uploader
	bucket   string
}

func NewS3Backend(client *s3.Client, bucket string) *S3Backend {
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		// Set part size to 5MB for multipart uploads
		u.PartSize = 5 * 1024 * 1024
		// Enable concurrency for large files
		u.Concurrency = 5
	})
	
	return &S3Backend{
		client:   client,
		uploader: uploader,
		bucket:   bucket,
	}
}

func (s3b *S3Backend) Put(ctx context.Context, key string, data io.Reader) error {
	// Use the uploader for streaming uploads without memory buffering
	result, err := s3b.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s3b.bucket),
		Key:    aws.String(key),
		Body:   data,
	})

	if err != nil {
		return fmt.Errorf("failed to put object %s: %w", key, err)
	}

	logging.Logf("[STORAGE] S3 Put: s3://%s/%s (ETag: %s)", s3b.bucket, key, aws.ToString(result.ETag))
	return nil
}

func (s3b *S3Backend) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	result, err := s3b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s3b.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		var notFound *types.NoSuchKey
		if errors.As(err, &notFound) {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, fmt.Errorf("failed to get object %s: %w", key, err)
	}

	return result.Body, nil
}

func (s3b *S3Backend) Delete(ctx context.Context, key string) error {
	_, err := s3b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s3b.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete object %s: %w", key, err)
	}

	// logging.Logf("[STORAGE] S3 Delete: s3://%s/%s", s3b.bucket, key)
	return nil
}

func (s3b *S3Backend) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string

	paginator := s3.NewListObjectsV2Paginator(s3b.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s3b.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects with prefix %s: %w", prefix, err)
		}

		for _, obj := range result.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
	}

	return keys, nil
}

func (s3b *S3Backend) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s3b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s3b.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		var notFound *types.NoSuchKey
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check existence of %s: %w", key, err)
	}

	return true, nil
}

func (s3b *S3Backend) Copy(ctx context.Context, srcKey, dstKey string) error {
	copySource := fmt.Sprintf("%s/%s", s3b.bucket, srcKey)

	_, err := s3b.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s3b.bucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(copySource),
	})

	if err != nil {
		return fmt.Errorf("failed to copy object from %s to %s: %w", srcKey, dstKey, err)
	}

	logging.Logf("[STORAGE] S3 Copy: s3://%s/%s -> s3://%s/%s", s3b.bucket, srcKey, s3b.bucket, dstKey)
	return nil
}

func (s3b *S3Backend) ListWithInfo(ctx context.Context, prefix string) ([]StorageInfo, error) {
	var infos []StorageInfo

	paginator := s3.NewListObjectsV2Paginator(s3b.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s3b.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		result, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects with prefix %s: %w", prefix, err)
		}

		for _, obj := range result.Contents {
			if obj.Key != nil {
				info := StorageInfo{
					Key:  *obj.Key,
					Size: *obj.Size,
				}

				if obj.LastModified != nil {
					info.LastModified = obj.LastModified.Format("2006-01-02T15:04:05Z")
				}

				infos = append(infos, info)
			}
		}
	}

	return infos, nil
}

func (s3b *S3Backend) GetInfo(ctx context.Context, key string) (*StorageInfo, error) {
	result, err := s3b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s3b.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		var notFound *types.NoSuchKey
		if errors.As(err, &notFound) {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, fmt.Errorf("failed to get info for %s: %w", key, err)
	}

	info := &StorageInfo{
		Key:  key,
		Size: *result.ContentLength,
	}

	if result.LastModified != nil {
		info.LastModified = result.LastModified.Format("2006-01-02T15:04:05Z")
	}

	return info, nil
}

func normalizeKey(key string) string {
	key = strings.TrimPrefix(key, "/")
	return strings.ReplaceAll(key, "\\", "/")
}
