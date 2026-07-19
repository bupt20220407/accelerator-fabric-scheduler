package observability

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"k8s.io/component-base/metrics/legacyregistry"
)

func Serve(ctx context.Context, address string) error {
	if address == "" {
		<-ctx.Done()
		return nil
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", legacyregistry.Handler())
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("ok\n"))
	})
	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve metrics on %s: %w", address, err)
	}
	return nil
}
