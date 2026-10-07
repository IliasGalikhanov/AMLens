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
		fmt.Fprintln(os.Stderr, "Использование: amlens serve|validate|analyze|template|demo [параметры]")
		return 2
	}
	command := args[0]
	if command != "serve" && command != "validate" && command != "analyze" && command != "template" && command != "demo" {
		fmt.Fprintln(os.Stderr, "Неизвестная команда")
		return 2
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	var dataDir, outputDir, envFile, address string
	var demo bool
	switch command {
	case "serve":
		flags.StringVar(&envFile, "env-file", "", "Явный путь к локальному .env")
		flags.StringVar(&address, "addr", "127.0.0.1:8000", "Адрес HTTP")
		flags.BoolVar(&demo, "demo", false, "Синтетические данные, только чтение, без ИИ")
	case "validate", "analyze":
		flags.StringVar(&dataDir, "data-dir", "data", "Каталог трёх Parquet-файлов")
		if command == "analyze" {
			flags.StringVar(&outputDir, "output-dir", "out", "Новый каталог CSV")
		}
	case "template", "demo":
		flags.StringVar(&outputDir, "output-dir", "data", "Каталог пустых Parquet-шаблонов")
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Лишние аргументы")
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
		fmt.Println("Созданы три Parquet-файла ("+command+"):", outputDir)
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
		fmt.Printf("Статус: OK\nУзлов: %d\nРёбер: %d\nТранзакций: %d\nSeed: %d\n", len(d.Nodes), len(d.Edges), len(d.Transactions), seeds)
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
		fmt.Printf("Статус: OK\nУзлов: %d\nРёбер: %d\nТранзакций: %d\nКластеров: %d\nCSV: %s\nВремя: %s\n", s.Nodes, s.Edges, s.Transactions, s.Clusters, outputDir, time.Since(started).Round(time.Millisecond))
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
		fmt.Fprintln(os.Stderr, "Не удалось открыть хранилище:", err)
		return 1
	}
	defer store.Close()
	if err = service.AttachStore(ctx, store); err != nil {
		fmt.Fprintln(os.Stderr, "Не удалось восстановить анализ:", err)
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
	fmt.Printf("AMLens API: http://%s\nAI настроен: %t\n", address, service.AIConfigured())
	err = server.ListenAndServe()
	stop()
	<-done
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "Не удалось запустить HTTP-сервер:", err)
		return 1
	}
	return 0
}
