package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pcloud "github.com/kopia-backends/go-pcloud"
)

func TestRunPCloudPreflightRequiredRootMissingFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"result":2005,"error":"not found"}`)) //nolint:errcheck
	}))
	defer server.Close()

	client := pcloud.NewClient(pcloud.Options{AccessToken: "token", APIHost: server.URL, HTTPClient: server.Client()})

	var out bytes.Buffer
	err := runPCloudPreflight(context.Background(), client, &out, "https://eapi.pcloud.com", []string{"/kopia-pcloud-prod"}, nil)
	if err == nil {
		t.Fatal("expected an error for a missing required root")
	}
	if !strings.Contains(err.Error(), "required folder missing: /kopia-pcloud-prod") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPCloudPreflightRequiredRootPresentSucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"result":0,"metadata":{"isfolder":true,"contents":[` + //nolint:errcheck
			`{"fileid":1,"name":"a.txt","path":"/root/a.txt","size":10},` +
			`{"folderid":2,"name":"sub","path":"/root/sub","isfolder":true}` +
			`]}}`))
	}))
	defer server.Close()

	client := pcloud.NewClient(pcloud.Options{AccessToken: "token", APIHost: server.URL, HTTPClient: server.Client()})

	var out bytes.Buffer
	if err := runPCloudPreflight(context.Background(), client, &out, "https://eapi.pcloud.com", []string{"/kopia-pcloud-prod"}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "required folder ok: /kopia-pcloud-prod files=1 folders=1 bytes=10") {
		t.Fatalf("unexpected output: %s", out.String())
	}
	if !strings.Contains(out.String(), "Preflight read-only checks passed.") {
		t.Fatalf("missing success line: %s", out.String())
	}
}

func TestRunPCloudPreflightOptionalTargetMissingWarnsOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"result":2005,"error":"not found"}`)) //nolint:errcheck
	}))
	defer server.Close()

	client := pcloud.NewClient(pcloud.Options{AccessToken: "token", APIHost: server.URL, HTTPClient: server.Client()})

	var out bytes.Buffer
	err := runPCloudPreflight(context.Background(), client, &out, "https://eapi.pcloud.com", nil, []string{"/kopia-pcloud-prod/sync-to-destination"})
	if err != nil {
		t.Fatalf("optional target missing must not fail preflight: %v", err)
	}
	if !strings.Contains(out.String(), "target folder missing: /kopia-pcloud-prod/sync-to-destination") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestInspectPCloudFolderRejectsUnsafePathBeforeNetworkCall(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"result":0,"metadata":{"isfolder":true,"contents":[]}}`)) //nolint:errcheck
	}))
	defer server.Close()

	client := pcloud.NewClient(pcloud.Options{AccessToken: "token", APIHost: server.URL, HTTPClient: server.Client()})

	var out bytes.Buffer
	for _, unsafe := range []string{"", "/", "."} {
		if err := inspectPCloudFolder(context.Background(), client, &out, "required", unsafe, true); err == nil {
			t.Fatalf("expected unsafe path %q to be rejected", unsafe)
		}
	}
	if calls != 0 {
		t.Fatalf("expected no HTTP calls for unsafe paths, got %d", calls)
	}
}

func TestSplitPCloudList(t *testing.T) {
	got := splitPCloudList([]string{"/a", "/b, /c", "", "  "})
	want := []string{"/a", "/b", "/c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestCleanSafePCloudPath(t *testing.T) {
	if _, err := cleanSafePCloudPath(""); err == nil {
		t.Fatal("expected empty path to be rejected")
	}
	if _, err := cleanSafePCloudPath("/"); err == nil {
		t.Fatal("expected root path to be rejected")
	}
	cleaned, err := cleanSafePCloudPath("relative/path/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleaned != "/relative/path" {
		t.Fatalf("unexpected cleaned path: %s", cleaned)
	}
}
