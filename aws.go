package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/xuri/excelize/v2"
)

const (
	region string = "us-east-1"
)

func createS3Client(region string) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	return s3.NewFromConfig(cfg), nil
}

type S3ClientAPI interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFins ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

func downloadFile(input *Input, client S3ClientAPI) (*excelize.File, error) {
	u, err := url.Parse(input.Filename)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "s3" {
		return nil, fmt.Errorf("file url %s must begin with s3://", input.Filename)
	}

	obj, err := downloadS3Object(u, client)
	if err != nil {
		return nil, fmt.Errorf("Error downloading file: %s", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(obj))
	if err != nil {
		return nil, fmt.Errorf("Error opening file after downloading it: %s", err)
	}

	dir, err := os.MkdirTemp(os.TempDir(), "macfin")
	if err != nil {
		return nil, err
	}

	filename := filepath.Join(dir, path.Base(u.Path))
	err = f.SaveAs(filename)
	if err != nil {
		return nil, fmt.Errorf("Error saving file to %s after downloading it: %s", filename, err)
	}
	return f, nil
}

func downloadS3Object(url *url.URL, client S3ClientAPI) ([]byte, error) {
	bucket := url.Host
	key := strings.TrimPrefix(url.Path, "/")

	resp, err := client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func uploadFile(f *excelize.File, bucket, key string, s3Client S3ClientAPI) error {
	fp, err := os.Open(f.Path)
	if err != nil {
		return fmt.Errorf("Error opening file: %s", err)
	}
	defer fp.Close()

	_, err = s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   fp,
	})
	if err != nil {
		return fmt.Errorf("Error uploading file to s3: %s", err)
	}
	return nil
}
