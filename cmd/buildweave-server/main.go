package main

import (
	"encoding/json"
	"fmt"
	"github.com/buildweave/buildweave/internal/api"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	state := os.Getenv("BUILDWEAVE_STATE")
	if state == "" {
		state = ".buildweave"
	}
	m, err := api.Open(state)
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok\n")) })
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(m.Metrics.Prometheus())) })
	mux.HandleFunc("/v1/status", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(m.Meta.Branches()) })
	addr := os.Getenv("BUILDWEAVE_ADDR")
	if addr == "" {
		addr = ":8787"
	}
	slog.Info("buildweave server listening", "addr", addr, "actor_id", "local-repository-actor")
	if err = http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
