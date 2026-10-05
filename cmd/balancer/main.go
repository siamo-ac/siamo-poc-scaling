// Command balancer is the demo load balancer:
//
//	GET /             -> round-robin proxy to backends
//	GET /route?key=K  -> consistent-hash (sticky) proxy for key K
//	GET /status       -> how many requests each backend has served
//
// Flags: -addr (default 127.0.0.1:18080),
//        -backends "http://h1:9001,http://h2:9002" (default 3 local backends).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/siamosystems/siamo-poc-scaling/internal/pool"
	"github.com/siamosystems/siamo-poc-scaling/internal/shard"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18080", "listen address")
	backends := flag.String("backends",
		"http://127.0.0.1:9001,http://127.0.0.1:9002,http://127.0.0.1:9003",
		"comma-separated backend URLs")
	flag.Parse()

	var bs []pool.Backend
	for i, u := range strings.Split(*backends, ",") {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		bs = append(bs, pool.Backend{ID: fmt.Sprintf("backend-%d", i+1), URL: u})
	}
	p, err := pool.New(bs)
	if err != nil {
		log.Fatal(err)
	}

	// Shard ring over backend ids (100 virtual nodes each).
	ids := make([]string, len(bs))
	for i, b := range bs {
		ids[i] = b.ID
	}
	ring := shard.New(ids, 100)
	byID := map[string]pool.Backend{}
	for _, b := range bs {
		byID[b.ID] = b
	}

	// Per-backend hit counters for the /status demo.
	var mu sync.Mutex
	hits := map[string]*atomic.Uint64{}
	for _, b := range bs {
		hits[b.ID] = &atomic.Uint64{}
	}
	count := func(id string) { hits[id].Add(1) }

	proxyTo := func(w http.ResponseWriter, r *http.Request, b pool.Backend) {
		target, err := url.Parse(b.URL)
		if err != nil {
			http.Error(w, "bad backend", http.StatusBadGateway)
			return
		}
		count(b.ID)
		w.Header().Set("X-Routed-To", b.ID) // who we chose, on top of backend's own X-Backend-Id
		httputil.NewSingleHostReverseProxy(target).ServeHTTP(w, r)
	}

	mux := http.NewServeMux()

	// Round-robin: each request goes to the next backend in turn.
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		proxyTo(w, r, p.Next())
	})

	// Shard router: key -> always the same backend.
	mux.HandleFunc("GET /route", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, `{"error":"?key= is required"}`, http.StatusBadRequest)
			return
		}
		id := ring.Route(key)
		w.Header().Set("X-Shard-Key", key)
		// Backends only serve /work; strip the routing path before proxying.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/work"
		r2.URL.RawQuery = ""
		proxyTo(w, r2, byID[id])
	})

	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		out := map[string]uint64{}
		for id, c := range hits {
			out[id] = c.Load()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"hits": out})
	})

	log.Printf("balancer on %s in front of %d backends", *addr, len(bs))
	log.Fatal(http.ListenAndServe(*addr, mux))
}
