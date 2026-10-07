package parquetio

import (
	"fmt"
	"github.com/parquet-go/parquet-go"
	"os"
	"path/filepath"
)

// These row types describe schemas, never customer records.
type NodeRow struct {
	GID   int64 `parquet:"gid"`
	Depth int8  `parquet:"depth"`
	Seed  bool  `parquet:"is_seed"`
}
type EdgeRow struct {
	Src    int64   `parquet:"src"`
	Dst    int64   `parquet:"dst"`
	Amount float64 `parquet:"sum_kzt"`
	Count  int64   `parquet:"n_tx"`
	Depth  int8    `parquet:"depth"`
}
type TransactionRow struct {
	Src    int64   `parquet:"src"`
	Dst    int64   `parquet:"dst"`
	Date   int32   `parquet:"date,date"`
	Amount float64 `parquet:"sum_kzt"`
}

func WriteTemplates(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	names := []string{"nodes.parquet", "edges.parquet", "transactions.parquet"}
	for _, name := range names {
		if _, err := os.Lstat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			return fmt.Errorf("%s: файл уже существует или недоступен", name)
		}
	}
	if err := writeEmpty[NodeRow](filepath.Join(dir, names[0])); err != nil {
		return err
	}
	if err := writeEmpty[EdgeRow](filepath.Join(dir, names[1])); err != nil {
		return err
	}
	return writeEmpty[TransactionRow](filepath.Join(dir, names[2]))
}
func writeEmpty[T any](path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	w := parquet.NewWriter(f, parquet.SchemaOf(new(T)))
	err = w.Close()
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
