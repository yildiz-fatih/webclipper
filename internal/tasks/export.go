package tasks

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/starwalkn/gotenberg-go-client/v8"
	"github.com/starwalkn/gotenberg-go-client/v8/document"
)

type Exporter struct {
	GotenbergClient *gotenberg.Client
	HttpClient      *http.Client
	PandocURL       string
}

const (
	TypeExport = "export"
)

type ExportPayload struct {
	Format    string `json:"format"`
	CleanHTML string `json:"clean_html"`
}

func (exp *Exporter) HandleExport(ctx context.Context, t *asynq.Task) error {
	payload := ExportPayload{}
	err := json.Unmarshal(t.Payload(), &payload)
	if err != nil {
		return err
	}

	switch payload.Format {
	case "pdf":
		// convert to pdf
		pdfReader, err := exp.htmlToPDF(payload.CleanHTML)
		if err != nil {
			return err
		}
		defer pdfReader.Close()
		/*
			// send the pdf as response
			w.Header().Set("Content-Type", "application/pdf")
			_, err = io.Copy(w, pdfReader)
			if err != nil {
				app.serverError(w, err)
				return
			}
		*/
	case "epub":
		// convert to epub
		epubReader, err := exp.htmlToEPUB(payload.CleanHTML)
		if err != nil {
			return err
		}
		defer epubReader.Close()
		/*
			// send the epub as response
			w.Header().Set("Content-Type", "application/epub+zip")
			_, err = io.Copy(w, epubReader)
			if err != nil {
				app.serverError(w, err)
				return
			}
		*/
	}

	return nil
}

func (exp *Exporter) htmlToPDF(htmlContent string) (io.ReadCloser, error) {
	// convert to pdf
	doc, err := document.FromString("index.html", htmlContent)
	if err != nil {
		return nil, err
	}

	res, err := exp.GotenbergClient.Send(context.Background(), gotenberg.NewHTMLRequest(doc))
	if err != nil {
		return nil, err
	}

	return res.Body, nil
}

func (exp *Exporter) htmlToEPUB(htmlContent string) (io.ReadCloser, error) {
	req, err := http.NewRequest("POST", exp.PandocURL+"/api/convert/from/html/to/epub", strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "text/html")
	req.Header.Set("Content-Disposition", `attachment; filename="index.html"`)

	res, err := exp.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}

	return res.Body, nil
}
