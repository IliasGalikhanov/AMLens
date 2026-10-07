package domain

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

// Money retains the decimal representation of each source float64, without
// introducing additional binary rounding during aggregation.
type Money struct{ decimal.Decimal }

func Amount(v float64) (Money, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return Money{}, fmt.Errorf("ожидается конечная неотрицательная сумма")
	}
	return Money{decimal.NewFromFloat(v)}, nil
}
func (m Money) Add(n Money) Money { return Money{m.Decimal.Add(n.Decimal)} }
func (m Money) Float() float64    { v, _ := m.Float64(); return v }
func (m Money) MarshalJSON() ([]byte, error) {
	if math.IsInf(m.Float(), 0) {
		return nil, fmt.Errorf("сумма превышает диапазон HTTP JSON")
	}
	return []byte(m.String()), nil
}
func ID(gid int64) string { return strconv.FormatInt(gid, 10) }

type Node struct {
	GID   int64
	Depth int
	Seed  bool
}
type Edge struct {
	Src, Dst int64
	Amount   Money
	Count    int64
	Depth    int
}
type Transaction struct {
	Src, Dst int64
	Date     time.Time
	Amount   Money
}
type Dataset struct {
	Nodes        []Node
	Edges        []Edge
	Transactions []Transaction
}
type Pair struct{ Src, Dst int64 }

func Validate(d Dataset) error {
	fail := func(table string, row int, field, message string) error {
		return fmt.Errorf("%s.parquet: строка %d, поле %s: %s", table, row+1, field, message)
	}
	gids := make(map[int64]bool, len(d.Nodes))
	for i, n := range d.Nodes {
		if gids[n.GID] {
			return fail("nodes", i, "gid", "повторный идентификатор")
		}
		gids[n.GID] = true
		if n.Depth < 0 || n.Depth > 4 {
			return fail("nodes", i, "depth", "допустимы значения 0–4")
		}
		if n.Seed != (n.Depth == 0) {
			return fail("nodes", i, "is_seed", "seed должен соответствовать depth=0")
		}
	}
	pairs := make(map[Pair]bool, len(d.Edges))
	for i, e := range d.Edges {
		if !gids[e.Src] || !gids[e.Dst] {
			return fail("edges", i, "src/dst", "узел отсутствует в nodes")
		}
		p := Pair{e.Src, e.Dst}
		if pairs[p] {
			return fail("edges", i, "src/dst", "повторная направленная пара")
		}
		pairs[p] = true
		if e.Amount.IsNegative() || math.IsInf(e.Amount.Float(), 0) {
			return fail("edges", i, "sum_kzt", "недопустимая сумма")
		}
		if e.Count <= 0 {
			return fail("edges", i, "n_tx", "ожидается положительное целое")
		}
		if e.Depth < 1 || e.Depth > 4 {
			return fail("edges", i, "depth", "допустимы значения 1–4")
		}
	}
	for i, t := range d.Transactions {
		if !gids[t.Src] || !gids[t.Dst] {
			return fail("transactions", i, "src/dst", "узел отсутствует в nodes")
		}
		if !pairs[Pair{t.Src, t.Dst}] {
			return fail("transactions", i, "src/dst", "пара отсутствует в edges")
		}
		if t.Amount.IsNegative() || math.IsInf(t.Amount.Float(), 0) {
			return fail("transactions", i, "sum_kzt", "недопустимая сумма")
		}
		if t.Date.Year() < 1 || t.Date.Year() > 9999 || t.Date.Hour() != 0 || t.Date.Minute() != 0 || t.Date.Second() != 0 || t.Date.Nanosecond() != 0 {
			return fail("transactions", i, "date", "ожидается календарная дата 0001–9999")
		}
	}
	return nil
}
