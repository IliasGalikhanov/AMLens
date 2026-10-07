package parquetio

import (
	"context"
	"finance.local/amlens/internal/domain"
	"fmt"
	"github.com/parquet-go/parquet-go"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Reader struct{}
type field struct{ name, kind string }

var schemas = map[string][]field{
	"nodes":        {{"gid", "int64"}, {"depth", "signed"}, {"is_seed", "bool"}},
	"edges":        {{"src", "int64"}, {"dst", "int64"}, {"sum_kzt", "float64"}, {"n_tx", "int64"}, {"depth", "int8"}},
	"transactions": {{"src", "int64"}, {"dst", "int64"}, {"date", "date"}, {"sum_kzt", "float64"}},
}
var rowLimits = map[string]int64{"nodes": 100000, "edges": 250000, "transactions": 1000000}

func (Reader) Read(ctx context.Context, dir string) (domain.Dataset, error) {
	d := domain.Dataset{}
	err := readTable(ctx, dir, "nodes", func(v []parquet.Value) error {
		depth := integer(v[1])
		d.Nodes = append(d.Nodes, domain.Node{GID: v[0].Int64(), Depth: int(depth), Seed: v[2].Boolean()})
		return nil
	})
	if err != nil {
		return d, err
	}
	err = readTable(ctx, dir, "edges", func(v []parquet.Value) error {
		amount, err := domain.Amount(v[2].Double())
		if err != nil {
			return fmt.Errorf("поле sum_kzt: %w", err)
		}
		d.Edges = append(d.Edges, domain.Edge{Src: v[0].Int64(), Dst: v[1].Int64(), Amount: amount, Count: v[3].Int64(), Depth: int(integer(v[4]))})
		return nil
	})
	if err != nil {
		return d, err
	}
	err = readTable(ctx, dir, "transactions", func(v []parquet.Value) error {
		amount, err := domain.Amount(v[3].Double())
		if err != nil {
			return fmt.Errorf("поле sum_kzt: %w", err)
		}
		day := time.Unix(int64(v[2].Int32())*86400, 0).UTC()
		d.Transactions = append(d.Transactions, domain.Transaction{Src: v[0].Int64(), Dst: v[1].Int64(), Date: day, Amount: amount})
		return nil
	})
	return d, err
}
func integer(v parquet.Value) int64 {
	if v.Kind() == parquet.Int32 {
		return int64(v.Int32())
	}
	return v.Int64()
}
func accepts(t parquet.Type, kind string) bool {
	logical := t.LogicalType()
	signed := func(bits int) bool {
		if logical == nil {
			return (bits == 64 && t.Kind() == parquet.Int64) || (bits == 32 && t.Kind() == parquet.Int32)
		}
		return logical.Integer != nil && logical.Integer.IsSigned && int(logical.Integer.BitWidth) == bits
	}
	switch kind {
	case "int64":
		return t.Kind() == parquet.Int64 && signed(64)
	case "int8":
		return t.Kind() == parquet.Int32 && signed(8)
	case "signed":
		return ((t.Kind() == parquet.Int32) && (signed(8) || signed(16) || signed(32))) || ((t.Kind() == parquet.Int64) && signed(64))
	case "bool":
		return t.Kind() == parquet.Boolean && logical == nil
	case "float64":
		return t.Kind() == parquet.Double && logical == nil
	case "date":
		return t.Kind() == parquet.Int32 && logical != nil && logical.Date != nil
	}
	return false
}
func readTable(ctx context.Context, dir, name string, consume func([]parquet.Value) error) (err error) {
	// Corrupt input must not terminate the HTTP process if a decoder panics.
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("%s.parquet: повреждённый Parquet", name)
		}
	}()
	f, err := os.Open(filepath.Join(dir, name+".parquet"))
	if err != nil {
		return fmt.Errorf("%s.parquet: файл недоступен", name)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("%s.parquet: файл недоступен", name)
	}
	if stat.Size() > 25<<20 {
		return fmt.Errorf("%s.parquet: превышен лимит 25 MiB", name)
	}
	pf, err := parquet.OpenFile(f, stat.Size())
	if err != nil {
		return fmt.Errorf("%s.parquet: не удалось прочитать Parquet", name)
	}
	if pf.NumRows() < 0 || pf.NumRows() > rowLimits[name] {
		return fmt.Errorf("%s.parquet: превышен лимит строк (%d)", name, rowLimits[name])
	}
	var total int64
	for _, g := range pf.Metadata().RowGroups {
		if g.TotalByteSize < 0 || g.TotalByteSize > (256<<20)-total {
			return fmt.Errorf("%s.parquet: превышен лимит распакованных данных 256 MiB", name)
		}
		total += g.TotalByteSize
	}
	required := schemas[name]
	indices := make([]int, len(required))
	for i, want := range required {
		count := 0
		for _, field := range pf.Schema().Fields() {
			if field.Name() == want.name {
				count++
			}
		}
		col, ok := pf.Schema().Lookup(want.name)
		if count != 1 || !ok || col.MaxRepetitionLevel != 0 || !accepts(col.Node.Type(), want.kind) {
			return fmt.Errorf("%s.parquet: поле %s: ожидается одна скалярная колонка %s", name, want.name, want.kind)
		}
		indices[i] = col.ColumnIndex
	}
	reader := parquet.NewReader(pf)
	defer reader.Close()
	rows := make([]parquet.Row, 128)
	rowNumber := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := reader.ReadRows(rows)
		for _, row := range rows[:n] {
			rowNumber++
			selected := make([]parquet.Value, len(required))
			counts := make([]int, len(required))
			for _, value := range row {
				for i, index := range indices {
					if value.Column() == index {
						selected[i] = value
						counts[i]++
					}
				}
			}
			for i, value := range selected {
				if counts[i] != 1 || value.IsNull() {
					return fmt.Errorf("%s.parquet: строка %d, поле %s: пропуск или повтор значения", name, rowNumber, required[i].name)
				}
			}
			if err := consume(selected); err != nil {
				return fmt.Errorf("%s.parquet: строка %d: %w", name, rowNumber, err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("%s.parquet: ошибка чтения строк", name)
		}
		if n == 0 {
			return fmt.Errorf("%s.parquet: чтение не продвигается", name)
		}
	}
	if int64(rowNumber) != pf.NumRows() {
		return fmt.Errorf("%s.parquet: число строк не совпадает с метаданными", name)
	}
	return nil
}
