package handlers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lispyclouds/climate"
	"github.com/revkeen/cli/internal/config"
	"github.com/revkeen/cli/internal/handlers"
	"github.com/spf13/cobra"
)

func TestGenericHandlerUsesV2AndAPIKeyFlag(t *testing.T) {
	var gotPath, gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("x-api-key")
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": []any{}})
	}))
	defer server.Close()

	handlers.HTTPClient = server.Client()
	t.Cleanup(func() { handlers.HTTPClient = http.DefaultClient })

	t.Setenv("HOME", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.Settings.BaseURL = server.URL
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	root := &cobra.Command{Use: "revkeen"}
	root.PersistentFlags().String("api-key", "", "")
	root.PersistentFlags().Bool("agent", false, "")
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("table", false, "")
	root.PersistentFlags().String("output", "json", "")
	_ = root.PersistentFlags().Set("api-key", "rk_test_flag")
	_ = root.PersistentFlags().Set("json", "true")

	cmd := &cobra.Command{Use: "list"}
	root.AddCommand(cmd)

	if err := handlers.GenericHandler(cmd, nil, climate.HandlerData{
		Method: "GET",
		Path:   "/customers",
	}); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if gotPath != "/v2/customers" {
		t.Fatalf("path = %q, want /v2/customers", gotPath)
	}
	if gotKey != "rk_test_flag" {
		t.Fatalf("api key = %q, want rk_test_flag", gotKey)
	}
}

func TestGenericHandlerSendsZeroLimit(t *testing.T) {
	var rawQuery, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	defer server.Close()

	handlers.HTTPClient = server.Client()
	t.Cleanup(func() { handlers.HTTPClient = http.DefaultClient })

	t.Setenv("HOME", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.Settings.BaseURL = server.URL
	_ = config.Save(cfg)

	root := &cobra.Command{Use: "revkeen"}
	root.PersistentFlags().String("api-key", "rk_x", "")
	root.PersistentFlags().Bool("agent", true, "")
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("table", false, "")
	root.PersistentFlags().String("output", "json", "")
	_ = root.PersistentFlags().Set("api-key", "rk_x")
	_ = root.PersistentFlags().Set("agent", "true")

	cmd := &cobra.Command{Use: "list"}
	root.AddCommand(cmd)
	cmd.Flags().Int("limit", 10, "")
	_ = cmd.Flags().Set("limit", "0")

	if err := handlers.GenericHandler(cmd, nil, climate.HandlerData{
		Method:      "get",
		Path:        "/customers",
		QueryParams: []climate.ParamMeta{{Name: "limit", Type: climate.Integer}},
	}); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v2/customers" {
		t.Fatalf("path = %q, want /v2/customers", gotPath)
	}
	if rawQuery != "limit=0" {
		t.Fatalf("query = %q, want limit=0", rawQuery)
	}
}
