package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"github.com/starwalkn/gotenberg-go-client/v8"
	"github.com/wneessen/go-mail"
	"github.com/yildiz-fatih/webclipper/internal/tasks"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	gotenbergURL := os.Getenv("GOTENBERG_URL")
	if gotenbergURL == "" {
		logger.Error("GOTENBERG_URL is not set")
		os.Exit(1)
	}

	pandocURL := os.Getenv("PANDOC_URL")
	if pandocURL == "" {
		logger.Error("PANDOC_URL is not set")
		os.Exit(1)
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		logger.Error("REDIS_URL is not set")
		os.Exit(1)
	}

	s3BucketName := os.Getenv("S3_BUCKET_NAME")
	if s3BucketName == "" {
		logger.Error("S3_BUCKET_NAME is not set")
		os.Exit(1)
	}

	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := mail.DefaultPortTLS // sensible default
	smtpPortStr := os.Getenv("SMTP_PORT")
	if smtpPortStr != "" {
		var err error
		smtpPort, err = strconv.Atoi(smtpPortStr)
		if err != nil {
			logger.Error("SMTP_PORT is not a valid integer")
			os.Exit(1)
		}
	}
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	smtpFrom := os.Getenv("SMTP_FROM")

	httpClient := &http.Client{Timeout: 30 * time.Second}

	gotenbergClient, err := gotenberg.NewClient(gotenbergURL, httpClient)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

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

	var mailClient *mail.Client
	if smtpHost != "" {
		mailClient, err = mail.NewClient(
			smtpHost,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithPort(smtpPort),
			mail.WithUsername(smtpUser),
			mail.WithPassword(smtpPass),
		)
		if err != nil {
			logger.Error(err.Error())
			os.Exit(1)
		}
	}

	clipper := &tasks.Clipper{
		GotenbergClient: gotenbergClient,
		HttpClient:      httpClient,
		PandocURL:       pandocURL,
		S3Client:        s3Client,
		S3Bucket:        s3BucketName,
		SMTPFrom:        smtpFrom,
		MailClient:      mailClient,
	}

	parsedRedisURL, err := url.Parse(redisURL)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	asynqServer := asynq.NewServer(asynq.RedisClientOpt{Addr: parsedRedisURL.Host}, asynq.Config{
		// Number of concurrent workers
		Concurrency: 10,
		// Queue priorities (higher number = higher priority)
		Queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},
	})

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeClipping, clipper.HandleClipping)

	logger.Info("starting worker", "type", tasks.TypeClipping)
	err = asynqServer.Run(mux)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
