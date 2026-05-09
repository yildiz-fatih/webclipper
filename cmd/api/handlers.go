package main

import (
	"net/http"
	"time"
)

func (app *application) getHealth(w http.ResponseWriter, r *http.Request) {
	type healthResponse struct {
		Status    string    `json:"status"`
		Timestamp time.Time `json:"timestamp"`
	}

	res := healthResponse{
		Status:    "pass",
		Timestamp: time.Now().UTC(),
	}

	err := writeJSON(w, http.StatusOK, nil, res)
	if err != nil {
		app.serverError(w, err)
		return
	}
}
