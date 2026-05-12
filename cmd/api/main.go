package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/hibiken/asynq"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/yildiz-fatih/webclipper/internal/models"
)

type application struct {
	logger      *slog.Logger
	clipModel   *models.ClipModel
	httpClient  *http.Client
	asynqClient *asynq.Client
}

func main() {
	const host = "0.0.0.0"

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = godotenv.Load()

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

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	logger.Info("connected to database")

	httpClient := &http.Client{Timeout: 30 * time.Second}

	parsedRedisURL, err := url.Parse(redisURL)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: parsedRedisURL.Host})
	defer asynqClient.Close()

	app := &application{
		logger:      logger,
		clipModel:   &models.ClipModel{DB: db},
		httpClient:  httpClient,
		asynqClient: asynqClient,
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
