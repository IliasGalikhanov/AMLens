// Package graph implements deterministic weighted Louvain modularity clustering.
package graph

import (
	"context"
	"finance.local/amlens/internal/domain"
	"math/rand"
	"sort"
)

type Louvain struct{}
type network []map[int]float64

func (Louvain) Groups(ctx context.Context, d domain.Dataset) ([][]int64, error) {
	ids := make([]int64, 0, len(d.Nodes))
	for _, n := range d.Nodes {
		ids = append(ids, n.GID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	index := map[int64]int{}
	members := make([][]int64, len(ids))
	g := make(network, len(ids))
	for i, id := range ids {
		index[id] = i
		members[i] = []int64{id}
		g[i] = map[int]float64{}
	}
	weights := map[domain.Pair]domain.Money{}
	maxWeight := domain.Money{}
	// Reciprocal edges share one undirected weight; loops occur once.
	for _, e := range d.Edges {
		x, y := e.Src, e.Dst
		if x > y {
			x, y = y, x
		}
		p := domain.Pair{Src: x, Dst: y}
		weights[p] = weights[p].Add(e.Amount)
		if weights[p].GreaterThan(maxWeight.Decimal) {
			maxWeight = weights[p]
		}
	}
	pairs := make([]domain.Pair, 0, len(weights))
	for p := range weights {
		pairs = append(pairs, p)
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Src != pairs[j].Src {
			return pairs[i].Src < pairs[j].Src
		}
		return pairs[i].Dst < pairs[j].Dst
	})
	if maxWeight.IsPositive() {
		for _, p := range pairs {
			v, _ := weights[p].DivRound(maxWeight.Decimal, 340).Float64()
			i, j := index[p.Src], index[p.Dst]
			if v > 0 {
				g[i][j] = v
				if i != j {
					g[j][i] = v
				}
			}
		}
	}
	rng := rand.New(rand.NewSource(42))
	for len(g) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		partition, err := move(ctx, g, rng)
		if err != nil {
			return nil, err
		}
		unique := map[int]int{}
		nextMembers := [][]int64{}
		for i, c := range partition {
			j, ok := unique[c]
			if !ok {
				j = len(unique)
				unique[c] = j
				nextMembers = append(nextMembers, []int64{})
			}
			nextMembers[j] = append(nextMembers[j], members[i]...)
		}
		if len(nextMembers) == len(g) {
			break
		}
		next := make(network, len(nextMembers))
		for i := range next {
			next[i] = map[int]float64{}
		}
		for i := range g {
			for _, j := range keys(g[i]) {
				if j < i {
					continue
				}
				v := g[i][j]
				a, b := unique[partition[i]], unique[partition[j]]
				next[a][b] += v
				if a != b {
					next[b][a] += v
				}
			}
		}
		g, members = next, nextMembers
	}
	for _, group := range members {
		sort.Slice(group, func(i, j int) bool { return group[i] < group[j] })
	}
	sort.Slice(members, func(i, j int) bool { return members[i][0] < members[j][0] })
	return members, nil
}
func keys(m map[int]float64) []int {
	k := make([]int, 0, len(m))
	for i := range m {
		k = append(k, i)
	}
	sort.Ints(k)
	return k
}
func move(ctx context.Context, g network, rng *rand.Rand) ([]int, error) {
	n := len(g)
	membership := make([]int, n)
	degree := make([]float64, n)
	totals := make([]float64, n)
	adj := make([][]int, n)
	m2 := 0.0
	for i := range g {
		membership[i] = i
		adj[i] = keys(g[i])
		for _, j := range adj[i] {
			degree[i] += g[i][j]
			if i == j {
				degree[i] += g[i][j]
			}
		}
		totals[i] = degree[i]
		m2 += degree[i]
	}
	if m2 == 0 {
		return membership, nil
	}
	order := rng.Perm(n)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		changed := false
		for _, i := range order {
			old := membership[i]
			weights := map[int]float64{old: 0}
			for _, j := range adj[i] {
				if j != i {
					weights[membership[j]] += g[i][j]
				}
			}
			totals[old] -= degree[i]
			best := old
			gain := weights[old] - degree[i]*totals[old]/m2
			for _, candidate := range keys(weights) {
				v := weights[candidate] - degree[i]*totals[candidate]/m2
				if v > gain+1e-12 {
					best, gain = candidate, v
				}
			}
			totals[best] += degree[i]
			membership[i] = best
			if best != old {
				changed = true
			}
		}
		if !changed {
			return membership, nil
		}
	}
}
