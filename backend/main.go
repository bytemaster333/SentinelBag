// @title           SentinelBag API
// @version         1.0
// @description     Deterministic wash trading detection for Solana tokens.
// @description     Analyzes up to 1,000 transactions via 3 concurrent heuristics:
// @description     Wallet Clustering (HHI), Circular Flow Detection, and Buyer Diversity Index.
// @description     Built for the Bags Hackathon — Colosseum Frontier 2025.
//
// @contact.name    SentinelBag
// @contact.url     https://github.com/bytemaster333/SentinelBag
//
// @license.name    MIT
// @license.url     https://opensource.org/licenses/MIT
//
// @host            localhost:8080
// @BasePath        /
//
// @externalDocs.description  GitHub Repository
// @externalDocs.url          https://github.com/bytemaster333/SentinelBag
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"

	"sentinelbag/cache"
	"sentinelbag/handlers"
	"sentinelbag/helius"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found — reading from environment")
	}

	heliusKey := os.Getenv("HELIUS_API_KEY")
	if heliusKey == "" {
		log.Fatal("HELIUS_API_KEY environment variable is required")
	}

	redisURL := envOr("REDIS_URL", "redis://localhost:6379")
	port := envOr("PORT", "8080")
	allowedOrigin := envOr("ALLOWED_ORIGIN", "http://localhost:3000")

	store, err := cache.NewStore(redisURL)
	if err != nil {
		log.Printf("WARNING: Redis unavailable (%v) — caching disabled", err)
		store = cache.NewNoopStore()
	}

	hClient := helius.NewClient(heliusKey)
	integrityH := handlers.NewIntegrityHandler(hClient, store)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{allowedOrigin},
		AllowedMethods: []string{"GET", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
		MaxAge:         300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"sentinelbag"}`))
	})

	// Core analysis endpoint
	r.Get("/api/integrity/{tokenAddress}", integrityH.GetIntegrityScore)

	// Swagger UI — activate by running: make swagger
	// r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))

	log.Printf("SentinelBag backend listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
