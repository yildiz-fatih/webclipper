package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hibiken/asynq"
	"github.com/starwalkn/gotenberg-go-client/v8"
	"github.com/starwalkn/gotenberg-go-client/v8/document"
)

type Exporter struct {
	GotenbergClient *gotenberg.Client
	HttpClient      *http.Client
	PandocURL       string
	S3Client        *s3.Client
	S3Bucket        string
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
		// save file in s3
		taskID, ok := asynq.GetTaskID(ctx)
		if !ok {
			return errors.New("could not get task id from context")
		}
		pdfBytes, err := io.ReadAll(pdfReader)
		if err != nil {
			return err
		}
		_, err = exp.S3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(exp.S3Bucket),
			Key:    aws.String(taskID + "." + payload.Format),
			Body:   bytes.NewReader(pdfBytes),
		})
		if err != nil {
			return err
		}
	case "epub":
		// convert to epub
		epubReader, err := exp.htmlToEPUB(payload.CleanHTML)
		if err != nil {
			return err
		}
		defer epubReader.Close()
		// save file in s3
		taskID, ok := asynq.GetTaskID(ctx)
		if !ok {
			return errors.New("could not get task id from context")
		}
		epubBytes, err := io.ReadAll(epubReader)
		if err != nil {
			return err
		}
		_, err = exp.S3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(exp.S3Bucket),
			Key:    aws.String(taskID + "." + payload.Format),
			Body:   bytes.NewReader(epubBytes),
		})
		if err != nil {
			return err
		}
	default:
		return errors.New("unsupported format: " + payload.Format)
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
