package publisher

import (
	"bytes"
	"cloud.google.com/go/pubsub"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"encoding/json"
	"fmt"
	"github.com/rismaster/allris-common/application"
	"github.com/rismaster/allris-common/common/slog"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	"net/http"
)

type PublishDoneRequest struct {
	Topic   string `json:"topic"`
	Message string `json:"message"`
}

func (p *PublishDoneRequest) SendDoneEvent(appContext *application.AppContext) {

	sData, err := json.Marshal(p)
	if err != nil {
		slog.Error("error parsing done event %+v", err)
		return
	}
	msg := &pubsub.Message{Data: sData}
	a := appContext.TopicDone.Publish(appContext.Ctx(), msg)
	id, err := a.Get(appContext.Ctx())
	if err != nil {
		slog.Error("error on Publish.Get: %v", err)
		return
	}

	url := appContext.Config.GetRestartUrl()

	slog.Info("URL:> %s", url)

	var jsonStr = []byte(`{"Data":"Done"}`)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	err, token := p.accessSecretVersion(appContext)
	if err != nil {
		slog.Error("error on Publish.Get: %v", err)
		return
	}

	req.Header.Set("X-Token", token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	slog.Info("response Status: %s -> Message published: %v", resp.Status, id)
}

func (p *PublishDoneRequest) accessSecretVersion(appContext *application.AppContext) (error, string) {

	// Create the client.
	client, err := secretmanager.NewClient(appContext.Ctx())
	if err != nil {
		return fmt.Errorf("failed to create secretmanager client: %v", err), ""
	}

	// Build the request.
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: appContext.Config.GetPublishDoneSecret(),
	}

	// Call the API.
	result, err := client.AccessSecretVersion(appContext.Ctx(), req)
	if err != nil {
		return fmt.Errorf("failed to access secret version: %v", err), ""
	}

	return nil, string(result.Payload.Data)
}
