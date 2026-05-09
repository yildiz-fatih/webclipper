package main

import (
	"log/slog"
	"net/http"
	"os"

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

	app := &application{
		logger: logger,
	}

	server := &http.Server{
		Addr:     host + ":" + port,
		Handler:  app.newRouter(),
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("starting server", "host", host, "port", port)
	err := server.ListenAndServe() // err is always non-nil
	logger.Error(err.Error())
	os.Exit(1)
}
