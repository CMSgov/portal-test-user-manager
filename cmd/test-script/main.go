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
	log.Printf("Starting...")

	bucket := os.Getenv("S3_BUCKET")
	log.Printf("S3 bucket: %q", bucket)

	key := os.Getenv("S3_KEY")
	log.Printf("S3 key: %q", key)

	log.Printf("Sheet password: %q", os.Getenv("SHEETPASSWORD"))

	file, err := os.Create(key)
	if err != nil {
		log.Fatalf("Unable to open file %q, %v", key, err)
	}
	defer file.Close()

	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	downloader := s3manager.NewDownloader(sess)
	numBytes, err := downloader.Download(file,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
	if err != nil {
		log.Fatalf("Unable to download item %q, %v", key, err)
	}
	fmt.Println("Downloaded", file.Name(), numBytes, "bytes")

	buf := new(bytes.Buffer)
	buf.ReadFrom(file)
	contents := buf.String()
	fmt.Printf("File contents: %s\n", contents)

	log.Printf("Exiting")
}
