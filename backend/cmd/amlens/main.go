package main

import (
	"context"
	"errors"
	"finance.local/amlens/internal/application"
	"finance.local/amlens/internal/config"
	"finance.local/amlens/internal/infrastructure/ai"
	"finance.local/amlens/internal/infrastructure/csvexport"
	"finance.local/amlens/internal/infrastructure/graph"
	"finance.local/amlens/internal/infrastructure/history"
	"finance.local/amlens/internal/infrastructure/parquetio"
	"finance.local/amlens/internal/infrastructure/storage"
	"finance.local/amlens/internal/presentation/httpapi"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() { os.Exit(run(os.Args[1:])) }
func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: amlens serve|validate|analyze|template|demo [options]")
		return 2
	}
	command := args[0]
	if command != "serve" && command != "validate" && command != "analyze" && command != "template" && command != "demo" {
		fmt.Fprintln(os.Stderr, "Unknown command")
		return 2
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	var dataDir, outputDir, envFile, address string
	var demo bool
	switch command {
	case "serve":
		flags.StringVar(&envFile, "env-file", "", "Explicit path to a local .env file")
		flags.StringVar(&address, "addr", "127.0.0.1:8000", "HTTP address")
		flags.BoolVar(&demo, "demo", false, "Read-only synthetic data, with AI disabled")
	case "validate", "analyze":
		flags.StringVar(&dataDir, "data-dir", "data", "Directory containing three Parquet files")
		if command == "analyze" {
			flags.StringVar(&outputDir, "output-dir", "out", "New CSV output directory")
		}
	case "template", "demo":
		flags.StringVar(&outputDir, "output-dir", "data", "Directory for empty Parquet templates")
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Unexpected arguments")
		return 2
	}
	if envFile != "" {
		if err := config.LoadEnv(envFile); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	if command == "template" || command == "demo" {
		write := parquetio.WriteTemplates
		if command == "demo" {
			write = parquetio.WriteDemo
		}
		if err := write(outputDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("Created three Parquet files ("+command+"):", outputDir)
		return 0
	}
	root := os.Getenv("ANALYSIS_STORAGE_DIR")
	if root == "" {
		root = "out/api"
	}
	var model application.Model
	settings := config.AIFromEnvironment()
	if settings.Configured() && !demo {
		model = ai.New(settings, nil)
	}
	service := application.New(parquetio.Reader{}, graph.Louvain{}, csvexport.Exporter{}, storage.Files{Root: root}, model)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if command == "validate" {
		d, err := service.Validate(ctx, dataDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		seeds := 0
		for _, n := range d.Nodes {
			if n.Seed {
				seeds++
			}
		}
		fmt.Printf("Status: OK\nNodes: %d\nEdges: %d\nTransactions: %d\nSeeds: %d\n", len(d.Nodes), len(d.Edges), len(d.Transactions), seeds)
		return 0
	}
	if command == "analyze" {
		started := time.Now()
		snap, err := service.Analyze(ctx, dataDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := csvexport.WriteDirectory(outputDir, snap.Exports); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		s := snap.Analysis.Summary
		fmt.Printf("Status: OK\nNodes: %d\nEdges: %d\nTransactions: %d\nClusters: %d\nCSV: %s\nTime: %s\n", s.Nodes, s.Edges, s.Transactions, s.Clusters, outputDir, time.Since(started).Round(time.Millisecond))
		return 0
	}
	database := os.Getenv("ANALYSIS_DATABASE")
	if database == "" {
		database = "out/analyses.db"
		if demo {
			database = "out/demo.db"
		}
	}
	store, err := history.Open(ctx, database, demo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not open storage:", err)
		return 1
	}
	defer store.Close()
	if err = service.AttachStore(ctx, store); err != nil {
		fmt.Fprintln(os.Stderr, "Could not restore analysis:", err)
		return 1
	}
	if demo {
		dir, err := os.MkdirTemp("", "amlens-demo-")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer os.RemoveAll(dir)
		if err = parquetio.WriteDemo(filepath.Join(dir, "data")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err = service.Seed(ctx, filepath.Join(dir, "data")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	server := &http.Server{Addr: address, Handler: httpapi.New(service, strings.Split(os.Getenv("CORS_ORIGINS"), ","), demo), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 180 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	fmt.Printf("AMLens API: http://%s\nAI configured: %t\n", address, service.AIConfigured())
	err = server.ListenAndServe()
	stop()
	<-done
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "Could not start HTTP server:", err)
		return 1
	}
	return 0
}
