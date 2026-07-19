package service

import (
	"context"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var r2Client *s3.Client
var r2Bucket string

func InitR2() {
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	r2Bucket = os.Getenv("R2_BUCKET_NAME")
	endpoint := os.Getenv("R2_ENDPOINT")

	if accountID == "" || accessKeyID == "" || secretAccessKey == "" || r2Bucket == "" {
		return
	}

	if endpoint == "" {
		endpoint = "https://" + accountID + ".r2.cloudflarestorage.com"
	}

	r2Client = s3.New(s3.Options{
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		BaseEndpoint: &endpoint,
	})
}

func IsR2Enabled() bool {
	return r2Client != nil
}

func R2Upload(ctx context.Context, key string, data io.Reader, contentType string) error {
	_, err := r2Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r2Bucket),
		Key:         aws.String(key),
		Body:        data,
		ContentType: aws.String(contentType),
	})
	return err
}

func R2Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	output, err := r2Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r2Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", err
	}
	ct := ""
	if output.ContentType != nil {
		ct = *output.ContentType
	}
	return output.Body, ct, nil
}

func R2Delete(ctx context.Context, key string) error {
	_, err := r2Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r2Bucket),
		Key:    aws.String(key),
	})
	return err
}
