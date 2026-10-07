package graph

import (
	"context"
	"finance.local/amlens/internal/domain"
	"reflect"
	"testing"
)

func TestCommunitiesWeightedStableAndComplete(t *testing.T) {
	d := domain.Dataset{}
	for i := int64(1); i <= 8; i++ {
		d.Nodes = append(d.Nodes, domain.Node{GID: i})
	}
	add := func(a, b int64, v float64) {
		m, _ := domain.Amount(v)
		d.Edges = append(d.Edges, domain.Edge{Src: a, Dst: b, Amount: m})
	}
	for _, base := range []int64{1, 4} {
		add(base, base+1, 100)
		add(base+1, base+2, 100)
		add(base, base+2, 100)
	}
	add(3, 4, 1)
	add(8, 8, 20)
	want := [][]int64{{1, 2, 3}, {4, 5, 6}, {7}, {8}}
	for pass := 0; pass < 4; pass++ {
		got, err := (Louvain{}).Groups(context.Background(), d)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v", got)
		}
		for i, j := 0, len(d.Nodes)-1; i < j; i, j = i+1, j-1 {
			d.Nodes[i], d.Nodes[j] = d.Nodes[j], d.Nodes[i]
		}
		for i, j := 0, len(d.Edges)-1; i < j; i, j = i+1, j-1 {
			d.Edges[i], d.Edges[j] = d.Edges[j], d.Edges[i]
		}
	}
}
func TestEmptyZeroWeightsAndCancellation(t *testing.T) {
	groups, err := (Louvain{}).Groups(context.Background(), domain.Dataset{})
	if err != nil || len(groups) != 0 {
		t.Fatal(groups, err)
	}
	d := domain.Dataset{Nodes: []domain.Node{{GID: 1}, {GID: 2}}, Edges: []domain.Edge{{Src: 1, Dst: 2}}}
	groups, err = (Louvain{}).Groups(context.Background(), d)
	if err != nil || len(groups) != 2 {
		t.Fatal(groups, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (Louvain{}).Groups(ctx, d); err == nil {
		t.Fatal("ignored cancellation")
	}
}
