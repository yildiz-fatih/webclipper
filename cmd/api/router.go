package main

import "net/http"

func (app *application) newRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", app.getHealth)
	mux.HandleFunc("GET /clips/{id}", app.getClip)
	mux.HandleFunc("POST /clips", app.postClip)
	mux.HandleFunc("POST /clips/{id}/export", app.postClipExport)

	return app.recoverPanic(mux)
}
