package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/starwalkn/gotenberg-go-client/v8"
	"github.com/starwalkn/gotenberg-go-client/v8/document"
	"github.com/yildiz-fatih/webclipper/internal/models"
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

	// get the clean html from database
	clip, err := app.clipModel.Get(id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			app.clientError(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
			return
		}
		app.serverError(w, err)
		return
	}

	switch req.Format {
	case "pdf":
		// convert to pdf
		pdfReader, err := app.htmlToPDF(clip.CleanHTML)
		if err != nil {
			app.serverError(w, err)
			return
		}
		defer pdfReader.Close()
		// send the pdf as response
		w.Header().Set("Content-Type", "application/pdf")
		_, err = io.Copy(w, pdfReader)
		if err != nil {
			app.serverError(w, err)
			return
		}
	case "epub":
		// convert to epub
		epubReader, err := app.htmlToEPUB(clip.CleanHTML)
		if err != nil {
			app.serverError(w, err)
			return
		}
		defer epubReader.Close()
		// send the epub as response
		w.Header().Set("Content-Type", "application/epub+zip")
		_, err = io.Copy(w, epubReader)
		if err != nil {
			app.serverError(w, err)
			return
		}
	}
}

func (app *application) htmlToPDF(htmlContent string) (io.ReadCloser, error) {
	// convert to pdf
	doc, err := document.FromString("index.html", htmlContent)
	if err != nil {
		return nil, err
	}

	res, err := app.gotenbergClient.Send(context.Background(), gotenberg.NewHTMLRequest(doc))
	if err != nil {
		return nil, err
	}

	return res.Body, nil
}

func (app *application) htmlToEPUB(htmlContent string) (io.ReadCloser, error) {
	req, err := http.NewRequest("POST", app.pandocURL+"/api/convert/from/html/to/epub", strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "text/html")
	req.Header.Set("Content-Disposition", `attachment; filename="index.html"`)

	res, err := app.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	return res.Body, nil
}
