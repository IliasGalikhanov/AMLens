package integration_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"finance.local/amlens/internal/application"
	"finance.local/amlens/internal/domain"
	"finance.local/amlens/internal/infrastructure/csvexport"
	"finance.local/amlens/internal/infrastructure/graph"
	"finance.local/amlens/internal/infrastructure/parquetio"
	"finance.local/amlens/internal/infrastructure/storage"
	"finance.local/amlens/internal/presentation/httpapi"
	"github.com/parquet-go/parquet-go"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const first int64 = 100000000000000001
const second int64 = 100000000000000002

func dataset(t *testing.T) map[string][]byte {
	t.Helper()
	dir := t.TempDir()
	if err := writeParquet(filepath.Join(dir, "nodes.parquet"), []parquetio.NodeRow{{GID: first, Depth: 0, Seed: true}, {GID: second, Depth: 1}, {GID: 3, Depth: 4}, {GID: 4, Depth: 0, Seed: true}}); err != nil {
		t.Fatal(err)
	}
	if err := writeParquet(filepath.Join(dir, "edges.parquet"), []parquetio.EdgeRow{{Src: first, Dst: second, Amount: 10000, Count: 2, Depth: 1}, {Src: second, Dst: first, Amount: 5000, Count: 1, Depth: 1}, {Src: first, Dst: 3, Amount: 5000, Count: 1, Depth: 4}}); err != nil {
		t.Fatal(err)
	}
	if err := writeParquet(filepath.Join(dir, "transactions.parquet"), []parquetio.TransactionRow{{Src: first, Dst: second, Date: 19000, Amount: 5000}, {Src: first, Dst: second, Date: 19000, Amount: 5000}, {Src: second, Dst: first, Date: 19000, Amount: 5000}, {Src: first, Dst: 3, Date: 19000, Amount: 5000}}); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, name := range []string{"nodes", "edges", "transactions"} {
		b, err := os.ReadFile(filepath.Join(dir, name+".parquet"))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = b
	}
	return files
}
func setup(t *testing.T, model application.Model, reader application.Reader, limit int64) (http.Handler, *application.Service, string) {
	t.Helper()
	root := t.TempDir()
	if reader == nil {
		reader = parquetio.Reader{}
	}
	svc := application.New(reader, graph.Louvain{}, csvexport.Exporter{}, storage.Files{Root: root, MaxFileBytes: limit}, model)
	return httpapi.New(svc, []string{"http://localhost:5173"}), svc, root
}
func upload(t *testing.T, h http.Handler, files map[string][]byte, names []string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for _, name := range names {
		part, err := w.CreateFormFile(name, name+".parquet")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(files[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/analyze", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
func request(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	h.ServeHTTP(rec, req)
	return rec
}
func requireStatus(t *testing.T, r *httptest.ResponseRecorder, status int) {
	t.Helper()
	if r.Code != status {
		t.Fatalf("HTTP %d expected %d: %s", r.Code, status, r.Body.String())
	}
}
func decode(t *testing.T, r *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

var fields = []string{"nodes", "edges", "transactions"}

func TestFullHTTPWorkflowAndFrontendContract(t *testing.T) {
	h, _, root := setup(t, nil, nil, 0)
	requireStatus(t, request(h, "GET", "/api/analysis", ""), 404)
	health := decode(t, request(h, "GET", "/api/health", ""))
	if health["analysis_ready"] != false || health["ai_configured"] != false {
		t.Fatal(health)
	}
	posted := upload(t, h, dataset(t), fields)
	requireStatus(t, posted, 200)
	id := decode(t, posted)["analysis_id"].(string)
	graph := request(h, "GET", "/api/analysis", "")
	requireStatus(t, graph, 200)
	analysis := decode(t, graph)
	if analysis["analysis_id"] != id {
		t.Fatal("version mismatch")
	}
	nodes := analysis["nodes"].([]any)
	if len(nodes) != 4 {
		t.Fatal("nodes lost")
	}
	ids := map[string]bool{}
	for _, raw := range nodes {
		node := raw.(map[string]any)
		gid, ok := node["gid"].(string)
		if !ok {
			t.Fatal("gid is not a string")
		}
		ids[gid] = true
	}
	if !ids[domain.ID(first)] || !ids[domain.ID(second)] {
		t.Fatal("long IDs changed")
	}
	card := request(h, "GET", "/api/nodes/"+domain.ID(first), "")
	requireStatus(t, card, 200)
	v := decode(t, card)
	if len(v["incoming"].([]any)) != 1 || len(v["outgoing"].([]any)) != 2 {
		t.Fatal("direction mismatch")
	}
	if len(v["data_gaps"].([]any)) != len(v["next_requests"].([]any)) {
		t.Fatal("advice mismatch")
	}
	requireStatus(t, request(h, "GET", "/api/nodes/9223372036854775808", ""), 422)
	requireStatus(t, request(h, "GET", "/api/nodes/999", ""), 404)
	for _, name := range []string{"nodes_roles.csv", "clusters.csv", "top_nodes.csv"} {
		r := request(h, "GET", "/api/exports/"+name, "")
		requireStatus(t, r, 200)
		if r.Header().Get("X-Analysis-Id") != id || !strings.HasPrefix(r.Header().Get("Content-Type"), "text/csv") {
			t.Fatal("export headers")
		}
		rows, err := csv.NewReader(bytes.NewReader(r.Body.Bytes())).ReadAll()
		if err != nil || len(rows) < 2 {
			t.Fatal(rows, err)
		}
		if name == "nodes_roles.csv" && !strings.Contains(r.Body.String(), domain.ID(first)) {
			t.Fatal("CSV lost int64")
		}
	}
	requireStatus(t, request(h, "GET", "/api/exports/secret.env", ""), 404)
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("uploaded inputs were retained", err)
	}
}
func TestInvalidUploadPreservesSnapshotAndCleansFiles(t *testing.T) {
	h, svc, root := setup(t, nil, nil, 0)
	files := dataset(t)
	requireStatus(t, upload(t, h, files, fields), 200)
	before, _ := svc.Current()
	for _, names := range [][]string{{"nodes", "edges"}, {"nodes", "edges", "transactions", "nodes"}, {"nodes", "edges", "transactions", "extra"}} {
		requireStatus(t, upload(t, h, files, names), 422)
	}
	corrupt := dataset(t)
	corrupt["nodes"] = []byte("not parquet")
	requireStatus(t, upload(t, h, corrupt, fields), 422)
	after, _ := svc.Current()
	if after.Analysis.ID != before.Analysis.ID {
		t.Fatal("failed upload replaced snapshot")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("failed uploads left data", err)
	}
	tiny, _, _ := setup(t, nil, nil, 8)
	requireStatus(t, upload(t, tiny, files, fields), 413)
	huge := httptest.NewRequest("POST", "/api/analyze", strings.NewReader(""))
	huge.ContentLength = (76 << 20) + 1
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, huge)
	requireStatus(t, rec, 413)
}

type slowReader struct{ started, release chan struct{} }

func (r slowReader) Read(ctx context.Context, dir string) (domain.Dataset, error) {
	close(r.started)
	select {
	case <-r.release:
	case <-ctx.Done():
		return domain.Dataset{}, ctx.Err()
	}
	return (parquetio.Reader{}).Read(ctx, dir)
}
func TestConcurrentAnalysisIsRejected(t *testing.T) {
	reader := slowReader{make(chan struct{}), make(chan struct{})}
	h, _, _ := setup(t, nil, reader, 0)
	files := dataset(t)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- upload(t, h, files, fields) }()
	<-reader.started
	requireStatus(t, upload(t, h, files, fields), 409)
	requireStatus(t, request(h, "GET", "/api/health", ""), 200)
	close(reader.release)
	requireStatus(t, <-done, 200)
}

type model struct {
	call func(context.Context, string, application.QuestionContext) (application.ModelAnswer, error)
}

func (model) Configured() bool { return true }
func (m model) Answer(ctx context.Context, q string, c application.QuestionContext) (application.ModelAnswer, error) {
	return m.call(ctx, q, c)
}
func question(id string) string {
	b, _ := json.Marshal(map[string]any{"analysis_id": id, "question": "Почему этот узел?", "context_gids": []string{domain.ID(first)}})
	return string(b)
}
func TestAskGroundingValidationAndUnavailable(t *testing.T) {
	calls := 0
	bad := false
	m := model{func(ctx context.Context, q string, c application.QuestionContext) (application.ModelAnswer, error) {
		calls++
		if len(c.Nodes) > 10 || len(c.Edges) > 10 {
			t.Fatal("unbounded context")
		}
		if bad {
			return application.ModelAnswer{Answer: "gid=999", GIDs: []string{domain.ID(first)}}, nil
		}
		return application.ModelAnswer{Answer: "Наблюдаются переводы; это гипотеза для проверки.", GIDs: []string{domain.ID(first)}}, nil
	}}
	h, svc, _ := setup(t, m, nil, 0)
	requireStatus(t, upload(t, h, dataset(t), fields), 200)
	snap, _ := svc.Current()
	q := question(snap.Analysis.ID)
	response := request(h, "POST", "/api/ask", q)
	requireStatus(t, response, 200)
	if len(decode(t, response)["references"].([]any)) != 1 {
		t.Fatal("missing references")
	}
	requireStatus(t, request(h, "POST", "/api/ask", question("stale")), 409)
	requireStatus(t, request(h, "POST", "/api/ask", strings.Replace(q, "\""+domain.ID(first)+"\"", domain.ID(first), 1)), 422)
	if calls != 1 {
		t.Fatal("invalid requests invoked model")
	}
	bad = true
	requireStatus(t, request(h, "POST", "/api/ask", q), 503)
	noAI, svc2, _ := setup(t, nil, nil, 0)
	requireStatus(t, upload(t, noAI, dataset(t), fields), 200)
	snap2, _ := svc2.Current()
	requireStatus(t, request(noAI, "POST", "/api/ask", question(snap2.Analysis.ID)), 503)
}
func TestAskChecksVersionAfterModelReturns(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	m := model{func(ctx context.Context, q string, c application.QuestionContext) (application.ModelAnswer, error) {
		close(started)
		<-release
		return application.ModelAnswer{Answer: "Факты", GIDs: []string{domain.ID(first)}}, nil
	}}
	h, svc, _ := setup(t, m, nil, 0)
	files := dataset(t)
	requireStatus(t, upload(t, h, files, fields), 200)
	snap, _ := svc.Current()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- request(h, "POST", "/api/ask", question(snap.Analysis.ID)) }()
	<-started
	requireStatus(t, upload(t, h, files, fields), 200)
	close(release)
	requireStatus(t, <-done, 409)
}
func TestCORSAndClientErrors(t *testing.T) {
	h, _, _ := setup(t, nil, nil, 0)
	req := httptest.NewRequest("OPTIONS", "/api/analyze", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	requireStatus(t, rec, 204)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatal("CORS")
	}
	req = httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Origin", "https://untrusted.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unexpected CORS")
	}
	for _, body := range []string{"{}", "null", `{"analysis_id":"a","question":"q","context_gids":[],"unexpected":true}`, `{"analysis_id":"a"}{}`} {
		requireStatus(t, request(h, "POST", "/api/ask", body), 422)
	}
	requireStatus(t, request(h, "POST", "/api/analyze", "not multipart"), 422)
}
func TestEmptyTemplatesThroughHTTP(t *testing.T) {
	dir := t.TempDir()
	if err := parquetio.WriteTemplates(dir); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, name := range fields {
		b, err := os.ReadFile(filepath.Join(dir, name+".parquet"))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = b
	}
	h, _, _ := setup(t, nil, nil, 0)
	requireStatus(t, upload(t, h, files, fields), 200)
	v := decode(t, request(h, "GET", "/api/analysis", ""))
	for _, key := range []string{"nodes", "edges", "clusters", "top_nodes"} {
		if a, ok := v[key].([]any); !ok || len(a) != 0 {
			t.Fatalf("%s must be [], got %v", key, v[key])
		}
	}
}

func writeParquet[T any](path string, rows []T) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := parquet.NewWriter(f, parquet.SchemaOf(new(T)))
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return w.Close()
}
