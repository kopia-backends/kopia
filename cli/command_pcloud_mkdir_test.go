package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	pcloud "github.com/kopia-backends/go-pcloud"
)

func TestEnsurePCloudFolderNoOpWhenFolderAlreadyExists(t *testing.T) {
	createCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/createfolder" {
			createCalls++
		}
		w.Write([]byte(`{"result":0,"metadata":{"isfolder":true,"contents":[]}}`)) //nolint:errcheck
	}))
	defer server.Close()

	client := pcloud.NewClient(pcloud.Options{AccessToken: "token", APIHost: server.URL, HTTPClient: server.Client()})

	if err := ensurePCloudFolder(context.Background(), client, "/kopia-pcloud-prod"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createCalls != 0 {
		t.Fatalf("expected no createfolder calls for an existing folder, got %d", createCalls)
	}
}

func TestEnsurePCloudFolderCreatesMissingSegments(t *testing.T) {
	var createdPaths []string
	listCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/listfolder":
			listCalls++
			if listCalls == 1 {
				w.Write([]byte(`{"result":2005,"error":"not found"}`)) //nolint:errcheck
				return
			}
			w.Write([]byte(`{"result":0,"metadata":{"isfolder":true,"contents":[]}}`)) //nolint:errcheck
		case "/createfolder":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			createdPaths = append(createdPaths, r.FormValue("path"))
			w.Write([]byte(`{"result":0,"metadata":{"isfolder":true}}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := pcloud.NewClient(pcloud.Options{AccessToken: "token", APIHost: server.URL, HTTPClient: server.Client()})

	if err := ensurePCloudFolder(context.Background(), client, "/kopia-pcloud-prod/mirrors/example"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"/kopia-pcloud-prod", "/kopia-pcloud-prod/mirrors", "/kopia-pcloud-prod/mirrors/example"}
	if len(createdPaths) != len(want) {
		t.Fatalf("createfolder calls = %v, want %v", createdPaths, want)
	}
	for i := range want {
		if createdPaths[i] != want[i] {
			t.Fatalf("createfolder calls = %v, want %v", createdPaths, want)
		}
	}
	// two listfolder calls: the initial existence check, and the final verify.
	if listCalls != 2 {
		t.Fatalf("expected 2 listfolder calls, got %d", listCalls)
	}
}

func TestEnsurePCloudFolderToleratesAlreadyExistsRace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/listfolder":
			w.Write([]byte(`{"result":2005,"error":"not found"}`)) //nolint:errcheck
		case "/createfolder":
			w.Write([]byte(`{"result":2004,"error":"already exists"}`)) //nolint:errcheck
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := pcloud.NewClient(pcloud.Options{AccessToken: "token", APIHost: server.URL, HTTPClient: server.Client()})

	// The final verification listfolder call will also report "not found"
	// from this handler, which is intentional: it proves ErrAlreadyExists on
	// createfolder does not short-circuit the verify step.
	err := ensurePCloudFolder(context.Background(), client, "/already-there")
	if err == nil {
		t.Fatal("expected the final verify listfolder call to surface an error")
	}
}

func TestCommandPCloudMkdirRejectsConfirmMismatchWithoutNetworkCall(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"result":0,"metadata":{"isfolder":true,"contents":[]}}`)) //nolint:errcheck
	}))
	defer server.Close()

	c := &commandPCloudMkdir{
		accessToken: "token",
		apiHost:     server.URL,
		path:        "/kopia-pcloud-prod",
		confirm:     "/not-the-same-path",
	}
	err := c.run(context.Background())
	if err == nil {
		t.Fatal("expected an error when --confirm does not match --path")
	}
	if calls != 0 {
		t.Fatalf("expected no HTTP calls on confirm mismatch, got %d", calls)
	}
}
