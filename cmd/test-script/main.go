package main

import (
	"log"
	"os"
)

func main() {
	log.Printf("Starting...")
	log.Printf("S3 bucket: %s", os.Getenv("S3_BUCKET"))
	log.Printf("S3 key: %s", os.Getenv("S3_KEY"))
	log.Printf("Exiting")
}
