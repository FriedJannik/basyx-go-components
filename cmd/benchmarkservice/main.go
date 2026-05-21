// Package main starts the BaSyx benchmark service.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/eclipse-basyx/basyx-go-components/internal/benchmark"
)

func main() {
	port := flag.Int("port", 8090, "Benchmark UI/API port")
	openAPIPath := flag.String("openapi", "cmd/submodelrepositoryservice/openapi.yaml", "Default OpenAPI spec path for request template generation")
	resultDir := flag.String("results", "benchmark-results", "Directory for persisted benchmark results")
	flag.Parse()

	store, err := benchmark.NewResultStore(*resultDir)
	if err != nil {
		log.Fatalf("Server error: %v", err)
	}
	manager := benchmark.NewManager(store)
	server := benchmark.NewServer(manager, *openAPIPath)
	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	log.Printf("BaSyx benchmark service listening on http://%s", addr)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
