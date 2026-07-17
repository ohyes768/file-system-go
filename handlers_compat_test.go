package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func setupTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	config.Storage.AudioDir = dir
	config.Storage.MaxUploadMB = 10
	fileLogger = consoleLogger

	r := mux.NewRouter()
	r.HandleFunc("/api/files", uploadHandler).Methods("POST")
	r.HandleFunc("/api/files/query", queryVideosHandler).Methods("POST")

	return httptest.NewServer(r), dir
}

func uploadFile(t *testing.T, serverURL, filename, body string, extract bool) UploadResponse {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, body); err != nil {
		t.Fatal(err)
	}
	if extract {
		if err := w.WriteField("extract", "true"); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	url := serverURL + "/api/files"
	if extract {
		url += "?extract=true"
	}
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestUploadWithoutExtract_NoExtractedDirSideEffect(t *testing.T) {
	server, dir := setupTestServer(t)
	defer server.Close()

	result := uploadFile(t, server.URL, "pkg.zip", "fake zip content", false)
	if !result.Success {
		t.Fatalf("upload failed: %+v", result)
	}
	if result.ExtractedDir != "" {
		t.Fatalf("expected no extracted_dir, got %q", result.ExtractedDir)
	}

	extractedPath := filepath.Join(dir, "pkg")
	if _, err := os.Stat(extractedPath); !os.IsNotExist(err) {
		t.Fatalf("expected no extracted directory at %s", extractedPath)
	}
}

func TestQueryVideos_UnchangedShape(t *testing.T) {
	server, dir := setupTestServer(t)
	defer server.Close()

	if err := os.WriteFile(filepath.Join(dir, "clip.mp4"), []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/files/query", strings.NewReader(`{"filters":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	queryVideosHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if _, ok := resp["success"]; !ok {
		t.Fatal("missing success field")
	}
	if _, ok := resp["videos"]; !ok {
		t.Fatal("missing videos field")
	}
}
