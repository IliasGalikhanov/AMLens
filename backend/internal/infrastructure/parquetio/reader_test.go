package parquetio

import (
	"context"
	"finance.local/amlens/internal/domain"
	"github.com/parquet-go/parquet-go"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmptyTemplatesAndNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	if err := WriteTemplates(dir); err != nil {
		t.Fatal(err)
	}
	d, err := (Reader{}).Read(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = domain.Validate(d); err != nil {
		t.Fatal(err)
	}
	if len(d.Nodes)+len(d.Edges)+len(d.Transactions) != 0 {
		t.Fatal("templates contain records")
	}
	if WriteTemplates(dir) == nil {
		t.Fatal("overwrote existing files")
	}
}
func write[T any](t *testing.T, dir, name string, rows []T) {
	t.Helper()
	f, err := os.Create(filepath.Join(dir, name+".parquet"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := parquet.NewWriter(f, parquet.SchemaOf(new(T)))
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}
func seedFiles(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "nodes", []NodeRow{{1, 0, true}, {2, 1, false}})
	write(t, dir, "edges", []EdgeRow{{1, 2, 12.5, 1, 1}})
	write(t, dir, "transactions", []TransactionRow{{1, 2, 0, 12.5}})
	return dir
}
func TestParquetOwnDataAndExtraColumns(t *testing.T) {
	dir := seedFiles(t)
	type extended struct {
		NodeRow
		Unused string `parquet:"unused"`
	}
	// Use flat fields: nested data must not stand in for mandatory scalars.
	type flat struct {
		GID    int64  `parquet:"gid"`
		Depth  int32  `parquet:"depth"`
		Seed   bool   `parquet:"is_seed"`
		Unused string `parquet:"unused"`
	}
	write(t, dir, "nodes", []flat{{1, 0, true, "ignored"}, {2, 1, false, "ignored"}})
	d, err := (Reader{}).Read(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = domain.Validate(d); err != nil {
		t.Fatal(err)
	}
	if d.Transactions[0].Date.Format("2006-01-02") != "1970-01-01" {
		t.Fatal("date conversion")
	}
}
func TestParquetBadSchemasAndNulls(t *testing.T) {
	cases := []struct {
		name   string
		change func(*testing.T, string)
	}{
		{"int32 gid", func(t *testing.T, d string) {
			type row struct {
				GID   int32 `parquet:"gid"`
				Depth int8  `parquet:"depth"`
				Seed  bool  `parquet:"is_seed"`
			}
			write(t, d, "nodes", []row{{1, 0, true}})
		}},
		{"unsigned gid", func(t *testing.T, d string) {
			type row struct {
				GID   uint64 `parquet:"gid"`
				Depth int8   `parquet:"depth"`
				Seed  bool   `parquet:"is_seed"`
			}
			write(t, d, "nodes", []row{{1, 0, true}})
		}},
		{"null gid", func(t *testing.T, d string) {
			type row struct {
				GID   *int64 `parquet:"gid,optional"`
				Depth int8   `parquet:"depth"`
				Seed  bool   `parquet:"is_seed"`
			}
			write(t, d, "nodes", []row{{nil, 0, true}})
		}},
		{"missing columns", func(t *testing.T, d string) {
			type row struct {
				GID int64 `parquet:"gid"`
			}
			write(t, d, "nodes", []row{{1}})
		}},
		{"edge int64 depth", func(t *testing.T, d string) {
			type row struct {
				Src    int64   `parquet:"src"`
				Dst    int64   `parquet:"dst"`
				Amount float64 `parquet:"sum_kzt"`
				Count  int64   `parquet:"n_tx"`
				Depth  int64   `parquet:"depth"`
			}
			write(t, d, "edges", []row{{1, 2, 1, 1, 1}})
		}},
		{"nan amount", func(t *testing.T, d string) { write(t, d, "edges", []EdgeRow{{1, 2, math.NaN(), 1, 1}}) }},
		{"infinite amount", func(t *testing.T, d string) { write(t, d, "edges", []EdgeRow{{1, 2, math.Inf(1), 1, 1}}) }},
		{"bad magic", func(t *testing.T, d string) {
			if err := os.WriteFile(filepath.Join(d, "nodes.parquet"), []byte("not parquet"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := seedFiles(t)
			tc.change(t, dir)
			_, err := (Reader{}).Read(context.Background(), dir)
			if err == nil || !strings.Contains(err.Error(), ".parquet") {
				t.Fatalf("expected safe file error, got %v", err)
			}
		})
	}
}
