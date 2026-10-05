// Command backend is one identical worker behind the load balancer.
// It answers /work and identifies itself in the X-Backend-Id header and
// the response body, so the demo can prove which server handled what.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:9001", "listen address")
	id := flag.String("id", "backend-1", "backend identity")
	flag.Parse()

	if envID := os.Getenv("BACKEND_ID"); envID != "" {
		*id = envID
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /work", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-Id", *id)
		json.NewEncoder(w).Encode(map[string]string{
			"backend_id": *id,
			"served_by":  r.Host,
			"path":       r.URL.Path,
		})
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	log.Printf("backend %s listening on %s", *id, *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
