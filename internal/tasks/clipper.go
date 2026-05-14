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
	"github.com/gosimple/slug"
	"github.com/hibiken/asynq"
	"github.com/starwalkn/gotenberg-go-client/v8"
	"github.com/starwalkn/gotenberg-go-client/v8/document"
	"github.com/wneessen/go-mail"
)

type Clipper struct {
	GotenbergClient *gotenberg.Client
	HttpClient      *http.Client
	PandocURL       string
	S3Client        *s3.Client
	S3Bucket        string
	SMTPFrom        string
	MailClient      *mail.Client
}

const (
	TypeClipping = "clipping"
)

type ClippingPayload struct {
	URL    string `json:"url"`
	Format string `json:"format"`
	Email  string `json:"email"`
}

func (c *Clipper) HandleClipping(ctx context.Context, t *asynq.Task) error {
	payload := ClippingPayload{}
	err := json.Unmarshal(t.Payload(), &payload)
	if err != nil {
		return err
	}

	if payload.Email != "" && c.MailClient == nil {
		return errors.New("SMTP is not configured")
	}

	// fetch and clean
	cleanHTML, title, err := c.fetchAndClean(payload.URL)
	if err != nil {
		return err
	}

	// convert
	fileBytes, err := c.convertTo(payload.Format, cleanHTML)
	if err != nil {
		return err
	}

	taskID, ok := asynq.GetTaskID(ctx)
	if !ok {
		return errors.New("could not get task id from context")
	}

	// upload
	_, err = c.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.S3Bucket),
		Key:         aws.String(taskID + "." + payload.Format),
		Body:        bytes.NewReader(fileBytes),
		ContentType: aws.String(contentTypeFor(payload.Format)),
	})
	if err != nil {
		return err
	}

	// email
	if payload.Email != "" {
		filename := fmt.Sprintf("%s.%s", slug.Make(title), payload.Format)
		err = c.sendEmail(payload.Email, title, filename, fileBytes)
		if err != nil {
			return err
		}
	}

	return nil
}

func contentTypeFor(format string) string {
	switch format {
	case "pdf":
		return "application/pdf"
	case "epub":
		return "application/epub+zip"
	case "html":
		return "text/html"
	default:
		return "application/octet-stream"
	}
}

func (c *Clipper) fetchAndClean(targetURL string) (cleanHTML string, title string, err error) {
	// get the html content of the url
	fetchRes, err := c.HttpClient.Get(targetURL)
	if err != nil {
		return "", "", err
	}
	defer fetchRes.Body.Close()
	// clean the html content
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return "", "", err
	}
	article, err := readability.FromReader(fetchRes.Body, parsedURL)
	if err != nil {
		return "", "", err
	}
	var buf bytes.Buffer
	err = article.RenderHTML(&buf)
	if err != nil {
		return "", "", err
	}
	clean := fmt.Sprintf("<html><head><title>%s</title></head><body>%s</body></html>", article.Title(), buf.String())

	return clean, article.Title(), nil
}

func (c *Clipper) convertTo(format string, cleanHTML string) ([]byte, error) {
	switch format {
	case "pdf":
		reader, err := c.htmlToPDF(cleanHTML)
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		fileBytes, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		return fileBytes, nil
	case "epub":
		reader, err := c.htmlToEPUB(cleanHTML)
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		fileBytes, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}

		return fileBytes, nil
	case "html":
		fileBytes := []byte(cleanHTML)
		return fileBytes, nil
	default:
		return nil, errors.New("unsupported format: " + format)
	}
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

func (c *Clipper) sendEmail(deliverTo string, title string, filename string, fileBytes []byte) error {
	message := mail.NewMsg()
	err := message.From(c.SMTPFrom)
	if err != nil {
		return err
	}
	err = message.To(deliverTo)
	if err != nil {
		return err
	}
	message.Subject("Your clipping is ready: " + title)
	err = message.AttachReader(filename, bytes.NewReader(fileBytes))
	if err != nil {
		return err
	}

	return c.MailClient.DialAndSend(message)
}
