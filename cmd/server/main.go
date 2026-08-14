package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	portfoliov1 "github.com/ananthv26-cog-demo-repos/rh-portfolio-go/gen/portfolio/v1"
	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/grpcapi"
	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/httpapi"
	"github.com/ananthv26-cog-demo-repos/rh-portfolio-go/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(context.Background()); err != nil {
		return err
	}

	positionStore := &store.PostgresStore{Pool: pool}
	httpServer := &http.Server{
		Addr:              ":" + getenv("PORT", "8081"),
		Handler:           httpapi.NewHandler(positionStore),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	grpcListener, err := net.Listen("tcp", ":"+getenv("GRPC_PORT", "9090"))
	if err != nil {
		return err
	}
	defer grpcListener.Close()
	grpcServer := grpc.NewServer()
	portfoliov1.RegisterPortfolioServiceServer(grpcServer, &grpcapi.Server{Store: positionStore})

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	go func() {
		log.Printf("HTTP listening on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server error: %v", err)
		}
	}()
	go func() {
		log.Printf("gRPC listening on %s", grpcListener.Addr())
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	<-signals
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
	grpcServer.GracefulStop()
	return nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
