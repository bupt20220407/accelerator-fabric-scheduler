package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestConsoleHandler(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("console fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(newConsoleHandler(directory))
	defer server.Close()

	client := server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	response, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusTemporaryRedirect || response.Header.Get("Location") != "/console/" {
		t.Fatalf("root response = %d %q", response.StatusCode, response.Header.Get("Location"))
	}

	console, err := server.Client().Get(server.URL + "/console/")
	if err != nil {
		t.Fatal(err)
	}
	defer console.Body.Close()
	if console.StatusCode != http.StatusOK {
		t.Fatalf("console status = %d", console.StatusCode)
	}
}

func TestValidateConsoleDirectory(t *testing.T) {
	if err := validateConsoleDirectory(t.TempDir()); err == nil {
		t.Fatal("validateConsoleDirectory accepted a directory without index.html")
	}
}
