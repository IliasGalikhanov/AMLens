package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"finance.local/amlens/internal/domain"
	"finance.local/amlens/internal/infrastructure/csvexport"
	"finance.local/amlens/internal/infrastructure/graph"
)

// Benchmark uses separate synthetic chain components. It measures domain
// validation, clustering, analysis and CSV, excluding Parquet, HTTP and SQLite.
func BenchmarkAnalysis(b *testing.B) {
	for _, size := range []int{1000, 5000, 10000} {
		b.Run(fmt.Sprintf("nodes_%d", size), func(b *testing.B) {
			d := domain.Dataset{}
			amount, _ := domain.Amount(12000.25)
			for i := 0; i < size; i++ {
				id := int64(9007199254741000 + i)
				seed := i%100 == 0
				depth := 1
				if seed {
					depth = 0
				}
				d.Nodes = append(d.Nodes, domain.Node{GID: id, Depth: depth, Seed: seed})
				if !seed {
					d.Edges = append(d.Edges, domain.Edge{Src: id - 1, Dst: id, Amount: amount, Count: 1, Depth: 1})
					d.Transactions = append(d.Transactions, domain.Transaction{Src: id - 1, Dst: id, Amount: amount, Date: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)})
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := domain.Validate(d); err != nil {
					b.Fatal(err)
				}
				groups, err := (graph.Louvain{}).Groups(context.Background(), d)
				if err != nil {
					b.Fatal(err)
				}
				a, err := domain.Analyze(d, groups)
				if err != nil {
					b.Fatal(err)
				}
				if _, err = (csvexport.Exporter{}).Encode(a); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
