package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

type application struct {
	logger          *slog.Logger
	httpClient      *http.Client
	asynqClient     *asynq.Client
	asynqInspector  *asynq.Inspector
	s3Client        *s3.Client
	s3PresignClient *s3.PresignClient
	s3Bucket        string
}

func main() {
	const host = "0.0.0.0"

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = godotenv.Load()

	s3BucketName := os.Getenv("S3_BUCKET_NAME")
	if s3BucketName == "" {
		logger.Error("S3_BUCKET_NAME is not set")
		os.Exit(1)
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		logger.Error("REDIS_URL is not set")
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		logger.Error("PORT is not set")
		os.Exit(1)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}

	parsedRedisURL, err := url.Parse(redisURL)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: parsedRedisURL.Host})
	defer asynqClient.Close()

	asynqInspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: parsedRedisURL.Host})

	sdkConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	s3Client := s3.NewFromConfig(sdkConfig, func(o *s3.Options) {
		o.DisableS3ExpressSessionAuth = aws.Bool(false)
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	s3PresignClient := s3.NewPresignClient(s3Client)

	app := &application{
		logger:          logger,
		httpClient:      httpClient,
		asynqClient:     asynqClient,
		asynqInspector:  asynqInspector,
		s3Client:        s3Client,
		s3PresignClient: s3PresignClient,
		s3Bucket:        s3BucketName,
	}

	server := &http.Server{
		Addr:     host + ":" + port,
		Handler:  app.newRouter(),
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("starting server", "host", host, "port", port)
	err = server.ListenAndServe() // err is always non-nil
	logger.Error(err.Error())
	os.Exit(1)
}
