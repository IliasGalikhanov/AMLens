package domain

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func money(v float64) Money {
	m, err := Amount(v)
	if err != nil {
		panic(err)
	}
	return m
}
func TestRoleRules(t *testing.T) {
	cases := []struct {
		name  string
		n     NodeResult
		role  string
		score float64
	}{
		{"coordinator wins", NodeResult{InDegree: 3, OutDegree: 5, SeedNeighbors: 2, Incoming: money(200000), Outgoing: money(100000)}, "coordinator", .7},
		{"collector exact threshold", NodeResult{InDegree: 3, OutDegree: 1, Incoming: money(100000), Outgoing: money(60000)}, "consolidator", .65},
		{"collector seed unknown balance", NodeResult{Seed: true, InDegree: 3, Incoming: money(100000), Outgoing: money(500000)}, "consolidator", .65},
		{"distributor", NodeResult{OutDegree: 5, Outgoing: money(100000)}, "distributor", .65},
		{"transit lower boundary", NodeResult{InDegree: 1, OutDegree: 1, Incoming: money(100), Outgoing: money(80)}, "transit", .7},
		{"transit upper boundary", NodeResult{InDegree: 1, OutDegree: 1, Incoming: money(100), Outgoing: money(120)}, "transit", .7},
		{"transit beyond exact boundary", NodeResult{InDegree: 1, OutDegree: 1, Incoming: money(100), Outgoing: money(120.000000001)}, "peripheral", .5},
		{"terminal", NodeResult{InDegree: 1, Depth: 2, Incoming: money(1)}, "terminal", .6},
		{"depth boundary", NodeResult{InDegree: 1, Depth: 4, Truncated: true, Incoming: money(1)}, "peripheral", .5},
		{"isolated", NodeResult{}, "peripheral", .5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			Classify(&tc.n)
			if tc.n.Role != tc.role || math.Abs(tc.n.RoleScore-tc.score) > 1e-12 {
				t.Fatalf("got %s %g", tc.n.Role, tc.n.RoleScore)
			}
			if len([]rune(tc.n.Evidence)) > 200 {
				t.Fatal("evidence too long")
			}
		})
	}
}
func sample() Dataset {
	return Dataset{Nodes: []Node{{100000000000000001, 0, true}, {100000000000000002, 1, false}, {3, 4, false}, {4, 0, true}},
		Edges:        []Edge{{100000000000000001, 100000000000000002, money(.1), 1, 1}, {100000000000000001, 3, money(.2), 1, 4}},
		Transactions: []Transaction{{100000000000000001, 100000000000000002, time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), money(.1)}}}
}
func TestAnalysisDecimalIDsDirectionsAndIsolates(t *testing.T) {
	d := sample()
	if err := Validate(d); err != nil {
		t.Fatal(err)
	}
	a, err := Analyze(d, [][]int64{{3, 100000000000000001, 100000000000000002}, {4}})
	if err != nil {
		t.Fatal(err)
	}
	if a.Summary.Volume.String() != "0.3" {
		t.Fatal(a.Summary.Volume)
	}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"gid":"100000000000000001"`) || !strings.Contains(string(b), `"gid":"100000000000000002"`) {
		t.Fatal("int64 IDs lost")
	}
	for _, n := range a.Nodes {
		if n.GID == 4 && (n.Priority != 0 || n.Role != "peripheral") {
			t.Fatalf("isolate: %+v", n)
		}
		if n.GID == 3 && !n.Truncated {
			t.Fatal("boundary")
		}
		if n.GID == 100000000000000001 && (n.Incoming.String() != "0" || n.Outgoing.String() != "0.3" || n.OutTx != 1) {
			t.Fatal("double counting or reversed direction")
		}
	}
}
func TestValidationRejectsBrokenReferencesAndValues(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Dataset)
	}{
		{"duplicate gid", func(d *Dataset) { d.Nodes = append(d.Nodes, d.Nodes[0]) }},
		{"seed depth", func(d *Dataset) { d.Nodes[0].Seed = false }},
		{"node depth", func(d *Dataset) { d.Nodes[0].Depth = 5 }},
		{"edge depth", func(d *Dataset) { d.Edges[0].Depth = 0 }},
		{"dangling edge", func(d *Dataset) { d.Edges[0].Dst = 999 }},
		{"duplicate edge", func(d *Dataset) { d.Edges = append(d.Edges, d.Edges[0]) }},
		{"nonpositive count", func(d *Dataset) { d.Edges[0].Count = 0 }},
		{"missing transaction pair", func(d *Dataset) { d.Transactions[0].Dst = 4 }},
		{"time instead of date", func(d *Dataset) { d.Transactions[0].Date = d.Transactions[0].Date.Add(time.Hour) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := sample()
			tc.mutate(&d)
			if Validate(d) == nil {
				t.Fatal("accepted invalid input")
			}
		})
	}
	for _, v := range []float64{math.NaN(), math.Inf(1), -1} {
		if _, err := Amount(v); err == nil {
			t.Fatal("accepted amount")
		}
	}
	if err := Validate(Dataset{}); err != nil {
		t.Fatal(err)
	}
}
func TestCardAdviceIsPairedAndDoesNotInventSourceCoverage(t *testing.T) {
	n := NodeResult{GID: 1, Depth: 4, Truncated: true, Incoming: money(1), InDegree: 1}
	card := NodeCard(Analysis{ID: "test", Edges: []EdgeResult{{Src: 2, Dst: 1, Amount: money(1), Count: 1}}}, n)
	if len(card.Incoming) != 1 || len(card.Outgoing) != 0 {
		t.Fatal("edge direction")
	}
	seen := map[string]bool{}
	for i, g := range card.Gaps {
		if seen[g.Code] || card.Requests[i].Code != g.Code {
			t.Fatal("advice mismatch")
		}
		seen[g.Code] = true
	}
	if !seen["DEPTH_BOUNDARY"] || !seen["COVERAGE_UNKNOWN"] || seen["INTRABANK_ONLY"] || seen["AMOUNT_THRESHOLD"] {
		t.Fatal(seen)
	}
}
