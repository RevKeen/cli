// Package listen implements `revkeen listen` / `revkeen trigger` (REV-2978).
package listen

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const signatureHeader = "X-Revkeen-Signature"

// ConnectedPayload is the first SSE `connected` frame from Engine.
type ConnectedPayload struct {
	SessionID     string   `json:"session_id"`
	SigningSecret string   `json:"signing_secret"`
	MerchantID    string   `json:"merchant_id"`
	Events        []string `json:"events"`
	TS            string   `json:"ts"`
}

// WebhookEvent is a forwarded webhook envelope.
type WebhookEvent struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Created   int64           `json:"created"`
	Livemode  bool            `json:"livemode"`
	Data      json.RawMessage `json:"data"`
	Triggered bool            `json:"triggered,omitempty"`
}

// SignPayload builds the RevKeen webhook signature header (t=,v1=).
func SignPayload(secret string, payload []byte, ts time.Time) string {
	unix := ts.Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d.", unix)
	_, _ = mac.Write(payload)
	return fmt.Sprintf("t=%d,v1=%s", unix, hex.EncodeToString(mac.Sum(nil)))
}

// ForwardPOSTs a webhook event to a local URL with production signature headers.
func Forward(ctx context.Context, client *http.Client, forwardTo string, secret string, event WebhookEvent) (int, error) {
	if client == nil {
		client = http.DefaultClient
	}
	body, err := json.Marshal(event)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, forwardTo, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "RevKeen-CLI-Listen/1.0")
	req.Header.Set("X-Revkeen-Event-Id", event.ID)
	req.Header.Set(signatureHeader, SignPayload(secret, body, time.Now()))

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// Auth holds credentials for Engine API calls.
type Auth struct {
	APIKey      string
	AccessToken string
	APIVersion  string
}

func (a Auth) apply(req *http.Request) {
	if a.APIVersion != "" {
		req.Header.Set("RevKeen-Version", a.APIVersion)
	} else {
		req.Header.Set("RevKeen-Version", "2026-05-01")
	}
	if a.APIKey != "" {
		req.Header.Set("x-api-key", a.APIKey)
		return
	}
	if a.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.AccessToken)
	}
}

// ListenURL builds GET /v2/cli/listen with optional event filters.
func ListenURL(origin string, events []string) string {
	base := strings.TrimRight(origin, "/") + "/v2/cli/listen"
	if len(events) == 0 {
		return base
	}
	q := url.Values{}
	q.Set("events", strings.Join(events, ","))
	return base + "?" + q.Encode()
}

// Trigger posts a fixture event type to Engine.
func Trigger(ctx context.Context, client *http.Client, origin string, auth Auth, eventType string, data map[string]any) (json.RawMessage, error) {
	if client == nil {
		client = http.DefaultClient
	}
	payload := map[string]any{"type": eventType}
	if data != nil {
		payload["data"] = data
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(origin, "/")+"/v2/cli/trigger", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	auth.apply(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("trigger failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

// SessionHandler is invoked when the connected frame arrives.
type SessionHandler func(ConnectedPayload) error

// EventHandler is invoked for each webhook SSE frame.
type EventHandler func(WebhookEvent) error

// RunSSE connects to the listen endpoint and dispatches frames until ctx cancels.
func RunSSE(ctx context.Context, client *http.Client, listenURL string, auth Auth, onSession SessionHandler, onEvent EventHandler) error {
	if client == nil {
		client = &http.Client{Timeout: 0} // long-lived stream
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listenURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	auth.apply(req)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("listen connect failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("listen connect failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return readSSE(ctx, resp.Body, onSession, onEvent)
}

func readSSE(ctx context.Context, r io.Reader, onSession SessionHandler, onEvent EventHandler) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var eventName string
	var dataLines []string

	flush := func() error {
		if len(dataLines) == 0 {
			eventName = ""
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = nil
		name := eventName
		eventName = ""

		switch name {
		case "connected":
			var payload ConnectedPayload
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				return fmt.Errorf("invalid connected frame: %w", err)
			}
			if onSession != nil {
				return onSession(payload)
			}
		case "webhook":
			var ev WebhookEvent
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				return fmt.Errorf("invalid webhook frame: %w", err)
			}
			if onEvent != nil {
				return onEvent(ev)
			}
		}
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			return flush()
		}

		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue // heartbeat / comment
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			continue
		}
		if strings.HasPrefix(line, "id:") {
			continue
		}
		if strings.HasPrefix(line, "retry:") {
			if _, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "retry:"))); err == nil {
				continue
			}
		}
	}
}
