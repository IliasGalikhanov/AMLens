package domain

import (
	"fmt"
	"github.com/shopspring/decimal"
	"math"
	"sort"
	"unicode/utf8"
)

type NodeResult struct {
	GID           int64   `json:"gid,string"`
	Depth         int     `json:"depth"`
	Seed          bool    `json:"is_seed"`
	Role          string  `json:"role"`
	RoleScore     float64 `json:"role_score"`
	ClusterID     int     `json:"cluster_id"`
	Priority      float64 `json:"priority_score"`
	Evidence      string  `json:"evidence"`
	InDegree      int     `json:"in_deg"`
	OutDegree     int     `json:"out_deg"`
	Incoming      Money   `json:"in_kzt"`
	Outgoing      Money   `json:"out_kzt"`
	Truncated     bool    `json:"truncated_by_depth"`
	InTx          int     `json:"-"`
	OutTx         int     `json:"-"`
	SeedNeighbors int     `json:"-"`
	Why           string  `json:"-"`
}
type EdgeResult struct {
	Src    int64 `json:"src,string"`
	Dst    int64 `json:"dst,string"`
	Amount Money `json:"sum_kzt"`
	Count  int64 `json:"n_tx"`
}
type Cluster struct {
	ID         int      `json:"cluster_id"`
	Nodes      int      `json:"n_nodes"`
	Seeds      int      `json:"n_seed"`
	Internal   Money    `json:"sum_kzt_internal"`
	TopGIDs    []string `json:"top_gids"`
	Hypothesis string   `json:"hypothesis"`
}
type TopNode struct {
	Rank     int     `json:"rank"`
	GID      int64   `json:"gid,string"`
	Role     string  `json:"role"`
	Priority float64 `json:"priority_score"`
	Why      string  `json:"why"`
}
type Summary struct {
	Nodes        int   `json:"n_nodes"`
	Edges        int   `json:"n_edges"`
	Transactions int   `json:"n_transactions"`
	Seeds        int   `json:"n_seed"`
	Clusters     int   `json:"n_clusters"`
	Volume       Money `json:"edge_volume_kzt"`
}
type Analysis struct {
	ID       string       `json:"analysis_id"`
	Summary  Summary      `json:"summary"`
	Nodes    []NodeResult `json:"nodes"`
	Edges    []EdgeResult `json:"edges"`
	Clusters []Cluster    `json:"clusters"`
	Top      []TopNode    `json:"top_nodes"`
}

func Classify(n *NodeResult) {
	ratioOK := !n.Seed && n.Incoming.IsPositive()
	ratio := decimal.Zero
	if ratioOK {
		ratio = n.Outgoing.Div(n.Incoming.Decimal)
	}
	less := func(x string) bool { return n.Outgoing.LessThanOrEqual(n.Incoming.Mul(decimal.RequireFromString(x))) }
	greater := func(x string) bool {
		return n.Outgoing.GreaterThanOrEqual(n.Incoming.Mul(decimal.RequireFromString(x)))
	}
	var reason string
	switch {
	case n.InDegree >= 3 && n.OutDegree >= 3 && n.SeedNeighbors >= 2:
		n.Role = "coordinator"
		n.RoleScore = math.Min(1, 0.6+0.05*float64(min(n.InDegree+n.OutDegree-6, 8)))
		reason = fmt.Sprintf("координации: входов=%d, выходов=%d, seed=%d", n.InDegree, n.OutDegree, n.SeedNeighbors)
	case n.InDegree >= 3 && n.Incoming.GreaterThanOrEqual(decimal.NewFromInt(100000)) && (n.Seed || (ratioOK && less("0.6"))):
		n.Role = "consolidator"
		n.RoleScore = math.Min(1, 0.65+0.05*float64(min(n.InDegree-3, 7)))
		reason = fmt.Sprintf("сбора: плательщиков=%d, вход=%.6g KZT", n.InDegree, n.Incoming.Float())
		if ratioOK {
			v, _ := ratio.Float64()
			reason += fmt.Sprintf(", out/in=%.1f%%", v*100)
		}
	case n.OutDegree >= 5 && n.Outgoing.GreaterThanOrEqual(decimal.NewFromInt(100000)):
		n.Role = "distributor"
		n.RoleScore = math.Min(1, 0.65+0.05*float64(min(n.OutDegree-5, 7)))
		reason = fmt.Sprintf("распределения: получателей=%d, выход=%.6g KZT", n.OutDegree, n.Outgoing.Float())
	case n.InDegree > 0 && n.OutDegree > 0 && ratioOK && greater("0.8") && less("1.2"):
		n.Role = "transit"
		v, _ := ratio.Float64()
		n.RoleScore = 0.9 - math.Abs(v-1)
		reason = fmt.Sprintf("транзита: входов=%d, выходов=%d, out/in=%.1f%%", n.InDegree, n.OutDegree, v*100)
	case n.InDegree > 0 && n.OutDegree == 0 && !n.Truncated:
		n.Role = "terminal"
		n.RoleScore = math.Min(0.85, 0.55+0.05*float64(min(n.InDegree, 6)))
		reason = fmt.Sprintf("получателя: вход=%.6g KZT, плательщиков=%d, выходов=0", n.Incoming.Float(), n.InDegree)
	default:
		n.Role = "peripheral"
		n.RoleScore = 0.5
		reason = fmt.Sprintf("явной роли не выявлены: входов=%d, выходов=%d", n.InDegree, n.OutDegree)
	}
	note := "; полный баланс неизвестен"
	if n.Truncated {
		note = "; depth=4: граница наблюдения, не доказанный сток"
	} else if n.Seed {
		note = "; seed: полнота входящих неизвестна, out/in не трактуется"
	}
	n.Evidence = "Признаки " + reason + note
	if utf8.RuneCountInString(n.Evidence) > 200 {
		n.Evidence = string([]rune(n.Evidence)[:199]) + "…"
	}
}

func Analyze(d Dataset, groups [][]int64) (Analysis, error) {
	a := Analysis{Nodes: []NodeResult{}, Edges: []EdgeResult{}, Clusters: []Cluster{}, Top: []TopNode{},
		Summary: Summary{Nodes: len(d.Nodes), Edges: len(d.Edges), Transactions: len(d.Transactions), Clusters: len(groups)}}
	byID := map[int64]*NodeResult{}
	seed := map[int64]bool{}
	neighbors := map[int64]map[int64]bool{}
	sorted := append([]Node(nil), d.Nodes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].GID < sorted[j].GID })
	for _, n := range sorted {
		a.Nodes = append(a.Nodes, NodeResult{GID: n.GID, Depth: n.Depth, Seed: n.Seed})
		seed[n.GID] = n.Seed
		if n.Seed {
			a.Summary.Seeds++
		}
	}
	for i := range a.Nodes {
		n := &a.Nodes[i]
		byID[n.GID] = n
		neighbors[n.GID] = map[int64]bool{}
	}
	for _, e := range d.Edges {
		src, dst := byID[e.Src], byID[e.Dst]
		src.OutDegree++
		dst.InDegree++
		src.Outgoing = src.Outgoing.Add(e.Amount)
		dst.Incoming = dst.Incoming.Add(e.Amount)
		neighbors[e.Src][e.Dst] = true
		neighbors[e.Dst][e.Src] = true
		a.Summary.Volume = a.Summary.Volume.Add(e.Amount)
		a.Edges = append(a.Edges, EdgeResult{e.Src, e.Dst, e.Amount, e.Count})
	}
	sort.Slice(a.Edges, func(i, j int) bool {
		if a.Edges[i].Src != a.Edges[j].Src {
			return a.Edges[i].Src < a.Edges[j].Src
		}
		return a.Edges[i].Dst < a.Edges[j].Dst
	})
	for _, t := range d.Transactions {
		byID[t.Src].OutTx++
		byID[t.Dst].InTx++
	}
	values := make([][4]float64, len(a.Nodes))
	maxima := [4]float64{}
	for i := range a.Nodes {
		n := &a.Nodes[i]
		for gid := range neighbors[n.GID] {
			if gid != n.GID && seed[gid] {
				n.SeedNeighbors++
			}
		}
		n.Truncated = n.Depth == 4 && n.OutDegree == 0
		volume := n.Incoming.Add(n.Outgoing).Float()
		if math.IsInf(volume, 0) || math.IsInf(a.Summary.Volume.Float(), 0) {
			return Analysis{}, fmt.Errorf("суммарный оборот превышает допустимый диапазон")
		}
		values[i] = [4]float64{math.Log1p(float64(n.InDegree + n.OutDegree)), math.Log1p(float64(n.InTx + n.OutTx)), math.Log1p(volume), math.Log1p(float64(n.SeedNeighbors))}
		for j, v := range values[i] {
			maxima[j] = math.Max(maxima[j], v)
		}
		Classify(n)
	}
	for i := range a.Nodes {
		n := &a.Nodes[i]
		contribution := [4]float64{}
		for j, w := range [4]float64{0.35, 0.25, 0.25, 0.15} {
			if maxima[j] > 0 {
				contribution[j] = w * values[i][j] / maxima[j]
			}
			n.Priority += contribution[j]
		}
		n.Priority = math.Min(1, math.Max(0, n.Priority))
		n.Why = fmt.Sprintf("Связей=%d (%.1f%% балла); операций=%d (%.1f%%); оборот=%.6g KZT (%.1f%%); соседей seed=%d (%.1f%%). Приоритет проверки, не вывод о виновности.", n.InDegree+n.OutDegree, 100*contribution[0], n.InTx+n.OutTx, 100*contribution[1], n.Incoming.Add(n.Outgoing).Float(), 100*contribution[2], n.SeedNeighbors, 100*contribution[3])
	}
	// The adapter returns a partition; reject omissions and duplicates.
	assigned := map[int64]bool{}
	for i, group := range groups {
		if len(group) == 0 {
			return Analysis{}, fmt.Errorf("пустой кластер")
		}
		for _, gid := range group {
			n, ok := byID[gid]
			if !ok || assigned[gid] {
				return Analysis{}, fmt.Errorf("некорректное разбиение графа")
			}
			assigned[gid] = true
			n.ClusterID = i + 1
		}
	}
	if len(assigned) != len(a.Nodes) {
		return Analysis{}, fmt.Errorf("разбиение не покрывает все узлы")
	}
	internal := make([]Money, len(groups))
	for _, e := range d.Edges {
		if byID[e.Src].ClusterID == byID[e.Dst].ClusterID {
			idx := byID[e.Src].ClusterID - 1
			internal[idx] = internal[idx].Add(e.Amount)
		}
	}
	for i, group := range groups {
		c := Cluster{ID: i + 1, Nodes: len(group), Internal: internal[i], TopGIDs: []string{}}
		members := make([]NodeResult, 0, len(group))
		counts := map[string]int{}
		for _, gid := range group {
			n := *byID[gid]
			members = append(members, n)
			if n.Seed {
				c.Seeds++
			}
			counts[n.Role]++
		}
		rank(members)
		for _, n := range members[:min(5, len(members))] {
			c.TopGIDs = append(c.TopGIDs, ID(n.GID))
		}
		if len(members) == 1 && members[0].InDegree+members[0].OutDegree == 0 {
			c.Hypothesis = "Изолированный узел: наблюдаемых связей=0; назначение группы не установлено"
		} else {
			c.Hypothesis = fmt.Sprintf("Группа наблюдаемых потоков: узлов=%d, seed=%d; признаки сбора у %d, распределения у %d, координации у %d; внутренний оборот=%.6g KZT. Гипотеза для проверки, не доказательство общей деятельности", c.Nodes, c.Seeds, counts["consolidator"], counts["distributor"], counts["coordinator"], c.Internal.Float())
		}
		a.Clusters = append(a.Clusters, c)
	}
	ranked := append([]NodeResult(nil), a.Nodes...)
	rank(ranked)
	for i, n := range ranked[:min(20, len(ranked))] {
		a.Top = append(a.Top, TopNode{i + 1, n.GID, n.Role, n.Priority, n.Why})
	}
	return a, nil
}
func rank(nodes []NodeResult) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Priority != nodes[j].Priority {
			return nodes[i].Priority > nodes[j].Priority
		}
		return nodes[i].GID < nodes[j].GID
	})
}
