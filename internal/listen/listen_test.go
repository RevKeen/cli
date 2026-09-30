package listen

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSignPayload(t *testing.T) {
	ts := time.Unix(1705689600, 0)
	payload := []byte(`{"id":"evt_1"}`)
	secret := "rk_wh_abc123"
	got := SignPayload(secret, payload, ts)

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("1705689600."))
	_, _ = mac.Write(payload)
	want := fmt.Sprintf("t=1705689600,v1=%s", hex.EncodeToString(mac.Sum(nil)))
	if got != want {
		t.Fatalf("signature mismatch:\n got %s\nwant %s", got, want)
	}
}

func TestListenURL(t *testing.T) {
	if got := ListenURL("https://api.revkeen.com", nil); got != "https://api.revkeen.com/v2/cli/listen" {
		t.Fatalf("got %s", got)
	}
	got := ListenURL("https://api.revkeen.com/", []string{"invoice.paid", "payment.succeeded"})
	if !strings.Contains(got, "events=invoice.paid") || !strings.Contains(got, "payment.succeeded") {
		t.Fatalf("unexpected url %s", got)
	}
}

func TestRunSSE_ConnectedAndWebhook(t *testing.T) {
	var forwarded []byte
	var mu sync.Mutex

	fwd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		forwarded = body
		mu.Unlock()
		if r.Header.Get("X-Revkeen-Signature") == "" {
			t.Error("missing signature header")
		}
		w.WriteHeader(200)
	}))
	defer fwd.Close()

	sse := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "rk_test_key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flush", 500)
			return
		}
		_, _ = fmt.Fprintf(w, "event: connected\ndata: {\"session_id\":\"cls_1\",\"signing_secret\":\"rk_wh_deadbeef\",\"merchant_id\":\"m1\"}\n\n")
		flusher.Flush()
		_, _ = fmt.Fprintf(w, "event: webhook\ndata: {\"id\":\"evt_1\",\"type\":\"invoice.paid\",\"created\":1,\"livemode\":false,\"data\":{\"object\":{\"id\":\"inv_1\"}}}\n\n")
		flusher.Flush()
	}))
	defer sse.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var secret string
	done := make(chan struct{})

	err := RunSSE(ctx, sse.Client(), sse.URL, Auth{APIKey: "rk_test_key"}, func(c ConnectedPayload) error {
		secret = c.SigningSecret
		return nil
	}, func(ev WebhookEvent) error {
		code, err := Forward(ctx, fwd.Client(), fwd.URL, secret, ev)
		if err != nil {
			return err
		}
		if code != 200 {
			return fmt.Errorf("forward status %d", code)
		}
		close(done)
		cancel()
		return nil
	})
	if err != nil && err != context.Canceled {
		t.Fatalf("RunSSE: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for forward")
	}

	mu.Lock()
	defer mu.Unlock()
	var parsed WebhookEvent
	if err := json.Unmarshal(forwarded, &parsed); err != nil {
		t.Fatalf("forward body: %v", err)
	}
	if parsed.ID != "evt_1" || parsed.Type != "invoice.paid" {
		t.Fatalf("unexpected event %#v", parsed)
	}
	if secret != "rk_wh_deadbeef" {
		t.Fatalf("secret %q", secret)
	}
}

func TestTrigger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/cli/trigger" {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["type"] != "payment.succeeded" {
			http.Error(w, "bad type", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	raw, err := Trigger(context.Background(), srv.Client(), srv.URL, Auth{APIKey: "k"}, "payment.succeeded", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"success":true`) {
		t.Fatalf("body %s", raw)
	}
}
