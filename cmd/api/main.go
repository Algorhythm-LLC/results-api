package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	port := os.Getenv("RESULTS_API_HTTP_PORT")
	if port == "" {
		port = "8082"
	}
	dsn := os.Getenv("RESULTS_API_CLICKHOUSE_DSN")
	if dsn == "" {
		dsn = "clickhouse://default:clickhouse@localhost:9009/default"
	}
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{dsnAddr(dsn)},
		Auth: clickhouse.Auth{
			Database: dsnDB(dsn),
			Username: dsnUser(dsn),
			Password: dsnPass(dsn),
		},
	})
	if err != nil {
		slog.Error("clickhouse connect failed", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := conn.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	summaryCache := newSummaryCacheFromEnv()
	if summaryCache != nil {
		slog.Info("results-api run summary cache enabled", "ttl", summaryCache.ttl, "max_entries", summaryCache.size)
	}

	r.Group(func(r chi.Router) {
		r.Use(newRateLimitMiddlewareFromEnv())
		if keys := parseAPIKeySet(os.Getenv("RESULTS_API_API_KEYS")); len(keys) > 0 {
			r.Use(apiKeyMiddleware(keys))
			slog.Info("results-api API key auth enabled", "keys", len(keys))
		}
		registerRoutes(r, conn, summaryCache)
	})

	srv := &http.Server{Addr: ":" + port, Handler: r}
	go func() {
		slog.Info("results-api starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func dsnAddr(dsn string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(dsn, "clickhouse://"), "tcp://")
	if i := strings.Index(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "localhost:9009"
	}
	return s
}

func dsnUser(dsn string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(dsn, "clickhouse://"), "tcp://")
	if i := strings.Index(s, "@"); i >= 0 {
		auth := s[:i]
		if j := strings.Index(auth, ":"); j >= 0 {
			return auth[:j]
		}
		return auth
	}
	return "default"
}

func dsnPass(dsn string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(dsn, "clickhouse://"), "tcp://")
	if i := strings.Index(s, "@"); i >= 0 {
		auth := s[:i]
		if j := strings.Index(auth, ":"); j >= 0 {
			return auth[j+1:]
		}
	}
	return "clickhouse"
}

func dsnDB(dsn string) string {
	if i := strings.LastIndex(dsn, "/"); i >= 0 && i < len(dsn)-1 {
		return dsn[i+1:]
	}
	return "default"
}

func registerRoutes(r chi.Router, conn driver.Conn, sc *summaryCache) {
	r.Get("/api/v1/runs/{run_id}/summary", getRunSummary(conn, sc))
	r.Get("/api/v1/runs/{run_id}/trades", getRunTrades(conn))
	r.Get("/api/v1/runs/{run_id}/equity-curve", getRunEquity(conn))
	r.Get("/api/v1/compare/runs", compareRuns(conn, sc))
	r.Get("/api/v1/compare/versions", compareVersions(conn, sc))
}
