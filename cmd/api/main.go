package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

type application struct {
	logger *slog.Logger
}

func main() {
	const host = "0.0.0.0"

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_ = godotenv.Load()

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

	app := &application{
		logger: logger,
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
