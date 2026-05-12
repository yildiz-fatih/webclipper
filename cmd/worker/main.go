package main

import (
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"github.com/starwalkn/gotenberg-go-client/v8"
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

	httpClient := &http.Client{Timeout: 30 * time.Second}

	gotenbergClient, err := gotenberg.NewClient(gotenbergURL, httpClient)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	exporter := &tasks.Exporter{
		GotenbergClient: gotenbergClient,
		HttpClient:      httpClient,
		PandocURL:       pandocURL,
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
	mux.HandleFunc(tasks.TypeExport, exporter.HandleExport)

	log.Println("Worker starting...")
	err = asynqServer.Run(mux)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
