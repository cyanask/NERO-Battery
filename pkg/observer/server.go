// NERO Battery modifications, 2026-11-16. Distributed under GPL v2; see docs/CHANGES.md.
// Package observer exposes only whitelisted IORegistry reads. It deliberately
// does not import the charging daemon, SMC, native power control or GUI packages.
package observer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/charlie0129/batt/pkg/localipc"
	"github.com/charlie0129/batt/pkg/powerinfo"
)

// Handler has no control routes, config writes, history writes or diagnostics.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("{\"read_only\":true,\"schema_version\":1}\n"))
	})
	mux.HandleFunc("GET /snapshot", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		snapshot, err := powerinfo.ReadObservation()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("{\"error\":\"battery_snapshot_unavailable\",\"read_only\":true}\n"))
			return
		}
		_ = json.NewEncoder(w).Encode(snapshot)
	})
	return mux
}

// Serve fails before creating an endpoint when policy or release identity is
// unavailable. It never installs a service or enters a charging state machine.
func Serve(ctx context.Context, socketPath string) error {
	policy, err := localipc.LoadPolicy(localipc.DefaultPolicyPath)
	if err != nil {
		return err
	}
	listener, err := localipc.Listen(socketPath, policy)
	if err != nil {
		return err
	}
	defer listener.Close()
	server := &http.Server{
		Handler: Handler(), ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second,
		IdleTimeout: 15 * time.Second, MaxHeaderBytes: 4096,
	}
	stopCtx, stop := context.WithCancel(ctx)
	defer stop()
	go func() {
		<-stopCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
