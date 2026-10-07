package parquetio

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/parquet-go/parquet-go"
)

// WriteDemo creates invented, deterministic records, with no source dataset.
// Components contain a hub, a chain, a cycle and an isolated node.
func WriteDemo(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	for _, name := range []string{"nodes.parquet", "edges.parquet", "transactions.parquet"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			return fmt.Errorf("%s: файл уже существует или недоступен", name)
		}
	}
	const base int64 = 9007199254741000
	nodes := []NodeRow{}
	edges := []EdgeRow{}
	transactions := []TransactionRow{}
	for component := 0; component < 3; component++ {
		start := base + int64(component*20)
		for i := 0; i < 20; i++ {
			depth := int8(1 + i%4)
			if i == 0 {
				depth = 0
			}
			nodes = append(nodes, NodeRow{GID: start + int64(i), Depth: depth, Seed: i == 0})
		}
		add := func(src, dst int, amount float64) {
			edges = append(edges, EdgeRow{Src: start + int64(src), Dst: start + int64(dst), Amount: amount, Count: 2, Depth: 1})
			for day := int32(0); day < 2; day++ {
				transactions = append(transactions, TransactionRow{Src: start + int64(src), Dst: start + int64(dst), Date: 20000 + day, Amount: amount / 2})
			}
		}
		for i := 1; i <= 9; i++ {
			add(0, i, float64(20000+i*4000))
			add(i, 10, float64(18000+i*3600))
		}
		for i := 10; i < 18; i++ {
			add(i, i+1, 120000)
		}
		add(18, 10, 120000)
	}
	if err := writeRows(filepath.Join(dir, "nodes.parquet"), nodes); err != nil {
		return err
	}
	if err := writeRows(filepath.Join(dir, "edges.parquet"), edges); err != nil {
		return err
	}
	return writeRows(filepath.Join(dir, "transactions.parquet"), transactions)
}
func writeRows[T any](path string, rows []T) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := parquet.NewWriter(file, parquet.SchemaOf(new(T)))
	for _, row := range rows {
		if err = writer.Write(row); err != nil {
			writer.Close()
			return err
		}
	}
	if err = writer.Close(); err != nil {
		return err
	}
	return file.Close()
}
