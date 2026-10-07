package integration_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"finance.local/amlens/internal/application"
	"finance.local/amlens/internal/infrastructure/history"
	"finance.local/amlens/internal/infrastructure/parquetio"
	"finance.local/amlens/internal/presentation/httpapi"
)

func TestPersistRestoreHistoryAndFailedImport(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "analyses.db")
	store, err := history.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	h, service, _ := setup(t, nil, nil, 0)
	if err = service.AttachStore(ctx, store); err != nil {
		t.Fatal(err)
	}
	requireStatus(t, upload(t, h, dataset(t), fields), 200)
	firstSnapshot, _ := service.Current()
	firstCard, _ := service.Card(first)
	requireStatus(t, upload(t, h, dataset(t), fields), 200)
	secondSnapshot, _ := service.Current()
	if firstSnapshot.Analysis.ID == secondSnapshot.Analysis.ID {
		t.Fatal("new analysis reuses ID")
	}
	bad := dataset(t)
	bad["nodes"] = []byte("not parquet")
	requireStatus(t, upload(t, h, bad, fields), 422)
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = history.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h, restored, _ := setup(t, nil, nil, 0)
	if err = restored.AttachStore(ctx, store); err != nil {
		t.Fatal(err)
	}
	current, _ := restored.Current()
	if !reflect.DeepEqual(secondSnapshot, current) {
		t.Fatal("restart changed snapshot, hidden evidence, decimal money or CSV")
	}
	response := request(h, "GET", "/api/analyses", "")
	requireStatus(t, response, 200)
	var records []application.AnalysisRecord
	if err = json.Unmarshal(response.Body.Bytes(), &records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].ID != secondSnapshot.Analysis.ID {
		t.Fatal(records)
	}
	requireStatus(t, request(h, "POST", "/api/analyses/"+firstSnapshot.Analysis.ID+"/activate", ""), 200)
	card, err := restored.Card(first)
	if err != nil || !reflect.DeepEqual(firstCard, card) {
		t.Fatal("restored card differs", err)
	}
	requireStatus(t, request(h, "POST", "/api/analyses/missing/activate", ""), 404)
	active, err := store.Current(ctx)
	if err != nil || active.Analysis.ID != firstSnapshot.Analysis.ID {
		t.Fatal("failed activation changed selection", err)
	}

	// A persistence failure must not publish a result visible only in RAM.
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	requireStatus(t, upload(t, h, dataset(t), fields), 500)
	current, _ = restored.Current()
	if current.Analysis.ID != firstSnapshot.Analysis.ID {
		t.Fatal("failed commit replaced current analysis")
	}
}

func TestSyntheticDemoIsReadOnlyAndCannotOpenPrivateStore(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "demo.db")
	store, err := history.Open(ctx, path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, service, _ := setup(t, nil, nil, 0)
	if err = service.AttachStore(ctx, store); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err = parquetio.WriteDemo(dir); err != nil {
		t.Fatal(err)
	}
	if err = parquetio.WriteDemo(dir); err == nil {
		t.Fatal("demo overwrote files")
	}
	if err = service.Seed(ctx, dir); err != nil {
		t.Fatal(err)
	}
	snap, _ := service.Current()
	if snap.Analysis.Summary.Nodes != 60 || snap.Analysis.Summary.Edges != 81 || snap.Analysis.Summary.Transactions != 162 {
		t.Fatal(snap.Analysis.Summary)
	}
	if snap.Analysis.Nodes[0].GID <= 1<<53 {
		t.Fatal("demo must exercise exact int64 identifiers")
	}
	card, err := service.Card(9007199254741019)
	if err != nil || len(card.Incoming)+len(card.Outgoing) != 0 {
		t.Fatal("isolated node lost")
	}
	h := httpapi.New(service, nil, true)
	for _, route := range []string{"/api/analyze", "/api/ask", "/api/analyses/" + snap.Analysis.ID + "/activate"} {
		requireStatus(t, request(h, "POST", route, "{}"), 403)
	}
	requireStatus(t, request(h, "GET", "/api/analysis", ""), 200)
	if err = service.Seed(ctx, dir); err != nil {
		t.Fatal(err)
	}
	again, _ := service.Current()
	if again.Analysis.ID != snap.Analysis.ID {
		t.Fatal("restart reseeds demo")
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	if wrong, err := history.Open(ctx, path, false); err == nil {
		wrong.Close()
		t.Fatal("private mode opened demo database")
	}
	privatePath := filepath.Join(t.TempDir(), "private.db")
	private, err := history.Open(ctx, privatePath)
	if err != nil {
		t.Fatal(err)
	}
	private.Close()
	if wrong, err := history.Open(ctx, privatePath, true); err == nil {
		wrong.Close()
		t.Fatal("demo mode opened private database")
	}
}
