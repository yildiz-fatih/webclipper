package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/starwalkn/gotenberg-go-client/v8"
	"github.com/yildiz-fatih/webclipper/internal/models"
)

type application struct {
	logger          *slog.Logger
	clipModel       *models.ClipModel
	gotenbergClient *gotenberg.Client
	httpClient      *http.Client
	pandocURL       string
}

func main() {
	const host = "0.0.0.0"

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = godotenv.Load()

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

	gotenbergClient, err := gotenberg.NewClient(gotenbergURL, httpClient)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	app := &application{
		logger:          logger,
		clipModel:       &models.ClipModel{DB: db},
		gotenbergClient: gotenbergClient,
		httpClient:      httpClient,
		pandocURL:       pandocURL,
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
