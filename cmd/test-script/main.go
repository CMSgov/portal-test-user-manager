package main

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func main() {
	region := "us-east-1"

	bucket := os.Getenv("S3_BUCKET")
	key := os.Getenv("S3_KEY")

	log.Printf("Starting application...")

	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)

	// download object from S3
	downloader := s3manager.NewDownloader(sess)
	file, err := os.Create(key)
	if err != nil {
		log.Fatalf("Error opening file %q: %s", key, err)
	}
	defer file.Close()
	_, err = downloader.Download(file,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
	if err != nil {
		log.Fatalf("Error downloading %q: %s", key, err)
	}
	fmt.Printf("Successfully downloaded %q from %q", key, bucket)

	// print object contents to verify
	buf := new(bytes.Buffer)
	buf.ReadFrom(file)
	fmt.Printf("File contents: %s", buf.String())

	// upload to s3
	uploader := s3manager.NewUploader(sess)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		log.Fatalf("Error uploading object %q: %s", key, err)
	}
	fmt.Printf("Successfully uploaded %q to %q", key, bucket)

	log.Printf("Exiting")
}
