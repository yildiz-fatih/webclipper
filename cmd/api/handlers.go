package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"codeberg.org/readeck/go-readability/v2"
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

func (app *application) getClip(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("not implemented yet"))
}

func (app *application) postClip(w http.ResponseWriter, r *http.Request) {
	// get the url from the request body
	type postClipRequest struct {
		URL string `json:"url"`
	}
	var req postClipRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		app.clientError(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		return
	}
	defer r.Body.Close()
	// get the html content of the url
	fetchRes, err := http.Get(req.URL)
	if err != nil {
		app.serverError(w, err)
		return
	}
	defer fetchRes.Body.Close()
	// clean the html content
	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		app.serverError(w, err)
		return
	}
	article, err := readability.FromReader(fetchRes.Body, parsedURL)
	if err != nil {
		app.serverError(w, err)
		return
	}
	var buf bytes.Buffer
	err = article.RenderHTML(&buf)
	if err != nil {
		app.serverError(w, err)
		return
	}
	cleanHTML := buf.String()
	// save to database
	clip, err := app.clipModel.Insert(req.URL, cleanHTML)
	if err != nil {
		app.serverError(w, err)
		return
	}
	// return the clip as json
	type postClipResponse struct {
		ID        string    `json:"id"`
		URL       string    `json:"url"`
		CleanHTML string    `json:"clean_html"`
		CreatedAt time.Time `json:"created_at"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	res := postClipResponse{
		ID:        clip.ID,
		URL:       clip.URL,
		CleanHTML: clip.CleanHTML,
		CreatedAt: clip.CreatedAt,
		ExpiresAt: clip.ExpiresAt,
	}
	err = writeJSON(w, http.StatusCreated, nil, res)
	if err != nil {
		app.serverError(w, err)
		return
	}
}
