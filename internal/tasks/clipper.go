package tasks

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

	"codeberg.org/readeck/go-readability/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hibiken/asynq"
	"github.com/starwalkn/gotenberg-go-client/v8"
	"github.com/starwalkn/gotenberg-go-client/v8/document"
)

type Clipper struct {
	GotenbergClient *gotenberg.Client
	HttpClient      *http.Client
	PandocURL       string
	S3Client        *s3.Client
	S3Bucket        string
}

const (
	TypeClipping = "clipping"
)

type ClippingPayload struct {
	URL    string `json:"url"`
	Format string `json:"format"`
}

func (c *Clipper) HandleClipping(ctx context.Context, t *asynq.Task) error {
	payload := ClippingPayload{}
	err := json.Unmarshal(t.Payload(), &payload)
	if err != nil {
		return err
	}

	// get the html content of the url
	fetchRes, err := c.HttpClient.Get(payload.URL)
	if err != nil {
		return err
	}
	defer fetchRes.Body.Close()
	// clean the html content
	parsedURL, err := url.Parse(payload.URL)
	if err != nil {
		return err
	}
	article, err := readability.FromReader(fetchRes.Body, parsedURL)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	err = article.RenderHTML(&buf)
	if err != nil {
		return err
	}
	cleanHTML := fmt.Sprintf("<html><head><title>%s</title></head><body>%s</body></html>", article.Title(), buf.String())

	switch payload.Format {
	case "pdf":
		// convert to pdf
		pdfReader, err := c.htmlToPDF(cleanHTML)
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
		_, err = c.S3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(c.S3Bucket),
			Key:    aws.String(taskID + "." + payload.Format),
			Body:   bytes.NewReader(pdfBytes),
		})
		if err != nil {
			return err
		}
	case "epub":
		// convert to epub
		epubReader, err := c.htmlToEPUB(cleanHTML)
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
		_, err = c.S3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(c.S3Bucket),
			Key:         aws.String(taskID + "." + payload.Format),
			Body:        bytes.NewReader(epubBytes),
			ContentType: aws.String("application/epub+zip"),
		})
		if err != nil {
			return err
		}
	default:
		return errors.New("unsupported format: " + payload.Format)
	}

	return nil
}

func (c *Clipper) htmlToPDF(htmlContent string) (io.ReadCloser, error) {
	// convert to pdf
	doc, err := document.FromString("index.html", htmlContent)
	if err != nil {
		return nil, err
	}

	res, err := c.GotenbergClient.Send(context.Background(), gotenberg.NewHTMLRequest(doc))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		return nil, errors.New("gotenberg failed with status code: " + res.Status)
	}

	return res.Body, nil
}

func (c *Clipper) htmlToEPUB(htmlContent string) (io.ReadCloser, error) {
	req, err := http.NewRequest("POST", c.PandocURL+"/api/convert/from/html/to/epub", strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "text/html")
	req.Header.Set("Content-Disposition", `attachment; filename="index.html"`)

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		return nil, errors.New("pandoc failed with status code: " + res.Status)
	}

	return res.Body, nil
}
