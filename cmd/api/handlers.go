package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/hibiken/asynq"
	"github.com/yildiz-fatih/webclipper/internal/models"
	"github.com/yildiz-fatih/webclipper/internal/tasks"
)

type clipResponse struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	CleanHTML string    `json:"clean_html"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

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
	// get the id from the url path
	id := r.PathValue("id")
	// get from database
	clip, err := app.clipModel.Get(id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			app.clientError(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
			return
		}
		app.serverError(w, err)
		return
	}
	// return the clip as json
	res := clipResponse{
		ID:        clip.ID,
		URL:       clip.URL,
		CleanHTML: clip.CleanHTML,
		CreatedAt: clip.CreatedAt,
		ExpiresAt: clip.ExpiresAt,
	}
	err = writeJSON(w, http.StatusOK, nil, res)
	if err != nil {
		app.serverError(w, err)
		return
	}
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
	fetchRes, err := app.httpClient.Get(req.URL)
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
	cleanHTML := fmt.Sprintf("<html><head><title>%s</title></head><body>%s</body></html>", article.Title(), buf.String())
	// save to database
	clip, err := app.clipModel.Insert(req.URL, cleanHTML)
	if err != nil {
		app.serverError(w, err)
		return
	}
	// return the clip as json
	res := clipResponse{
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

func (app *application) postClipExport(w http.ResponseWriter, r *http.Request) {
	// get the id from the url path
	id := r.PathValue("id")
	// get the format from the request body
	type postClipExportRequest struct {
		Format string `json:"format"`
	}
	var req postClipExportRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		app.clientError(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		return
	}
	defer r.Body.Close()
	// validate the format
	if req.Format != "pdf" && req.Format != "epub" {
		app.clientError(w, http.StatusBadRequest, "unsupported format")
		return
	}

	// get clip from database
	clip, err := app.clipModel.Get(id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			app.clientError(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
			return
		}
		app.serverError(w, err)
		return
	}

	payload := tasks.ExportPayload{
		Format:    req.Format,
		CleanHTML: clip.CleanHTML,
	}
	payloadJson, err := json.Marshal(payload)
	if err != nil {
		app.serverError(w, err)
		return
	}

	// enqueue the task
	taskInfo, err := app.asynqClient.Enqueue(asynq.NewTask(tasks.TypeExport, payloadJson))
	if err != nil {
		app.serverError(w, err)
		return
	}
	app.logger.Info("enqueued task", "id", taskInfo.ID, "type", taskInfo.Type, "queue", taskInfo.Queue)
	// return immediately
	type postExportResponse struct {
		ExportID string `json:"export_id"`
		Status   string `json:"status"`
	}
	res := postExportResponse{
		ExportID: taskInfo.ID,
		Status:   "pending",
	}
	err = writeJSON(w, http.StatusAccepted, nil, res)
	if err != nil {
		app.serverError(w, err)
		return
	}
}

func (app *application) getExport(w http.ResponseWriter, r *http.Request) {
	// get the id from the url path
	id := r.PathValue("id")
	fmt.Println(id)
	fmt.Println("not implemented yet")
}
