package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	address := flag.String("address", "0.0.0.0:8080", "HTTP listen address")
	directory := flag.String("directory", "frontend/dist", "compiled console directory")
	flag.Parse()

	if err := validateConsoleDirectory(*directory); err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:    *address,
		Handler: newConsoleHandler(*directory),
	}
	log.Printf("serving accelerator fabric console on http://%s/console/", *address)
	log.Fatal(server.ListenAndServe())
}

func validateConsoleDirectory(directory string) error {
	index := filepath.Join(directory, "index.html")
	if info, err := os.Stat(index); err != nil || info.IsDir() {
		return fmt.Errorf("compiled console index is unavailable at %s", index)
	}
	return nil
}

func newConsoleHandler(directory string) http.Handler {
	mux := http.NewServeMux()
	files := http.StripPrefix("/console/", http.FileServer(http.Dir(directory)))

	mux.Handle("/console/", files)
	mux.HandleFunc("/healthz", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" {
			http.NotFound(response, request)
			return
		}
		http.Redirect(response, request, "/console/", http.StatusTemporaryRedirect)
	})
	return mux
}
