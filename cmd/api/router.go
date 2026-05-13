package main

import "net/http"

func (app *application) newRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", app.getHealth)
	mux.HandleFunc("POST /clippings", app.postClipping)
	mux.HandleFunc("GET /clippings/{id}", app.getClipping)

	return app.recoverPanic(mux)
}
