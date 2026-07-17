package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractZip_RejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")
	createZip(t, zipPath, map[string]string{"../escape.txt": "pwned"})

	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatal(err)
	}
	err := ExtractArchive(zipPath, dest)
	if err == nil {
		t.Fatal("expected path traversal error")
	}
}

func TestExtractZip_OK(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "pkg.zip")
	createZip(t, zipPath, map[string]string{"a.txt": "hello"})
	dest := filepath.Join(dir, "pkg")
	_ = os.MkdirAll(dest, 0755)
	if err := ExtractArchive(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dest, "a.txt"))
	if err != nil || string(b) != "hello" {
		t.Fatalf("got %q err=%v", b, err)
	}
}

func TestExtractTarGz_OK(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "pkg.tar.gz")
	createTarGz(t, tarPath, map[string]string{"a.txt": "hello"})
	dest := filepath.Join(dir, "pkg")
	_ = os.MkdirAll(dest, 0755)
	if err := ExtractArchive(tarPath, dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dest, "a.txt"))
	if err != nil || string(b) != "hello" {
		t.Fatalf("got %q err=%v", b, err)
	}
}

func TestExtractTarGz_RejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "evil.tar.gz")
	createTarGz(t, tarPath, map[string]string{"../escape.txt": "pwned"})
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ExtractArchive(tarPath, dest); err == nil {
		t.Fatal("expected path traversal error")
	}
}

func TestArchiveBaseName(t *testing.T) {
	cases := map[string]string{
		"foo.zip":         "foo",
		"foo.tar":         "foo",
		"foo.tar.gz":      "foo",
		"foo.tgz":         "foo",
		"path/to/foo.zip": "foo",
		"bar.mp4":         "",
	}
	for in, want := range cases {
		if got := ArchiveBaseName(in); got != want {
			t.Fatalf("%s: got %q want %q", in, got, want)
		}
	}
}

func createZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func createTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(body)),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tw, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
}
