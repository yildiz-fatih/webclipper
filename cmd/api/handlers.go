package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hibiken/asynq"
	"github.com/yildiz-fatih/webclipper/internal/tasks"
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

func (app *application) postClipping(w http.ResponseWriter, r *http.Request) {
	type postClippingRequest struct {
		URL    string `json:"url"`
		Format string `json:"format"`
	}
	var req postClippingRequest
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
	// enqueue the task
	payload := tasks.ClippingPayload{
		URL:    req.URL,
		Format: req.Format,
	}
	payloadJson, err := json.Marshal(payload)
	if err != nil {
		app.serverError(w, err)
		return
	}

	taskInfo, err := app.asynqClient.Enqueue(asynq.NewTask(tasks.TypeClipping, payloadJson), asynq.Retention(24*time.Hour))
	if err != nil {
		app.serverError(w, err)
		return
	}
	app.logger.Info("enqueued task", "id", taskInfo.ID, "type", taskInfo.Type, "queue", taskInfo.Queue)
	// return immediately
	type postClippingResponse struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	res := postClippingResponse{
		ID:     taskInfo.ID,
		Status: "pending",
	}
	err = writeJSON(w, http.StatusAccepted, nil, res)
	if err != nil {
		app.serverError(w, err)
		return
	}

}

func (app *application) getClipping(w http.ResponseWriter, r *http.Request) {
	// get the id from the url path
	id := r.PathValue("id")
	// check task status
	taskInfo, err := app.asynqInspector.GetTaskInfo("default", id)
	if err != nil {
		app.serverError(w, err)
		return
	}
	switch taskInfo.State {
	case asynq.TaskStateCompleted:
		// get the file from s3 and return as response
		var payload tasks.ClippingPayload
		err := json.Unmarshal(taskInfo.Payload, &payload)
		if err != nil {
			app.serverError(w, err)
			return
		}

		request, err := app.s3PresignClient.PresignGetObject(r.Context(), &s3.GetObjectInput{
			Bucket: aws.String(app.s3Bucket),
			Key:    aws.String(id + "." + payload.Format),
		}, func(opts *s3.PresignOptions) {
			opts.Expires = time.Duration(1 * time.Hour)
		})
		if err != nil {
			app.serverError(w, err)
			return
		}
		// return the presigned url as response
		writeJSON(w, 200, nil, map[string]string{
			"download_url": request.URL,
		})
		return
	case asynq.TaskStateArchived:
		writeJSON(w, http.StatusInternalServerError, nil, map[string]string{
			"status": "failed",
		})
		return
	default:
		writeJSON(w, http.StatusAccepted, nil, map[string]string{
			"status": "pending",
		})
		return
	}
}
