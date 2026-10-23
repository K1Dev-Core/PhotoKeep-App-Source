package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"photokeep/services/ai-gateway/internal/gateway"
)

func main() {
	analyzer, concurrency, err := gateway.NewAnalyzerFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	address := strings.TrimSpace(os.Getenv("PHOTOKEEP_GATEWAY_LISTEN"))
	if address == "" {
		address = "127.0.0.1:8081"
	}
	server := &http.Server{
		Addr:              address,
		Handler:           gateway.New(analyzer, os.Getenv("PHOTOKEEP_GATEWAY_TOKEN"), concurrency).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("PhotoKeep AI gateway provider=%s listen=%s concurrency=%d", safeProviderName(), address, concurrency)
	log.Fatal(server.ListenAndServe())
}

func safeProviderName() string {
	provider := strings.TrimSpace(os.Getenv("PHOTOKEEP_GATEWAY_PROVIDER"))
	if provider == "" {
		return "xkiro"
	}
	return provider
}
