package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Payload struct {
	Text string `json:"text"`
}

type SlackPublisher struct {
	WebhookURL string
}

func NewSlackPublisher(webhookURL string) *SlackPublisher {
	return &SlackPublisher{WebhookURL: webhookURL}
}

func (s *SlackPublisher) Send(ctx context.Context, message string) error {
	payload := Payload{Text: message}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx,
		http.MethodPost,
		s.WebhookURL,
		bytes.NewBuffer(jsonPayload),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send webhook: %s (status: %d)", string(body), resp.StatusCode)
	}

	return nil
}
