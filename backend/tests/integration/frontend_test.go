package integration_test

import (
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRealFrontendClientAgainstGoHTTP(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js unavailable; run with Node 24+ to verify the frontend client")
	}
	h, _, _ := setup(t, nil, nil, 0)
	server := httptest.NewServer(h)
	defer server.Close()
	dir := t.TempDir()
	for name, data := range dataset(t) {
		if err := os.WriteFile(filepath.Join(dir, name+".parquet"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	script := filepath.Join("..", "..", "..", "scripts", "test-api-contract.mjs")
	cmd := exec.CommandContext(ctx, node, script, server.URL, dir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("frontend contract failed: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}
