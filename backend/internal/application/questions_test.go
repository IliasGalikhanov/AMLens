package application

import (
	"encoding/json"
	"finance.local/amlens/internal/domain"
	"testing"
)

func TestContextCapsAndLargestOutgoingFirst(t *testing.T) {
	a := domain.Analysis{Nodes: []domain.NodeResult{{GID: 1}, {GID: 2}}}
	for i := int64(3); i < 40; i++ {
		a.Nodes = append(a.Nodes, domain.NodeResult{GID: i})
		amount, _ := domain.Amount(float64(100 - i))
		a.Edges = append(a.Edges, domain.EdgeResult{Src: 1, Dst: i, Amount: amount, Count: 1})
	}
	amount, _ := domain.Amount(1)
	a.Edges = append(a.Edges, domain.EdgeResult{Src: 2, Dst: 39, Amount: amount, Count: 1})
	c, err := BuildContext(a, []int64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Nodes) > 7 || len(c.Edges) > 10 || c.Nearby != 38 {
		t.Fatal("limits", len(c.Nodes), len(c.Edges), c.Nearby)
	}
	if c.Edges[0].Src != 1 || c.Edges[1].Src != 2 {
		t.Fatal("largest outgoing per selection must come first")
	}
	b, err := json.Marshal(c)
	if err != nil || len(b) > 32<<10 {
		t.Fatal("byte limit", err)
	}
}
func TestModelReferencesMustBeGrounded(t *testing.T) {
	c := QuestionContext{Nodes: []ContextNode{{NodeResult: domain.NodeResult{GID: 1}}, {NodeResult: domain.NodeResult{GID: 100000000000000001}}}}
	cases := []ModelAnswer{
		{Answer: "", GIDs: []string{"1"}},
		{Answer: "a", GIDs: []string{}},
		{Answer: "a", GIDs: []string{"999"}},
		{Answer: "a", GIDs: []string{"1", "1"}},
		{Answer: "gid=999", GIDs: []string{"1"}},
		{Answer: "100000000000000001", GIDs: []string{"1"}},
	}
	for _, a := range cases {
		if validateAnswer(a, c) == nil {
			t.Fatal("invalid answer accepted")
		}
	}
	if err := validateAnswer(ModelAnswer{Answer: "gid=1: наблюдаемые переводы", GIDs: []string{"1"}}, c); err != nil {
		t.Fatal(err)
	}
}
