package csvexport

import (
	"bytes"
	"encoding/csv"
	"finance.local/amlens/internal/domain"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Exporter struct{}

func number(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
func (Exporter) Encode(a domain.Analysis) (map[string][]byte, error) {
	nodes := [][]string{{"gid", "role", "role_score", "cluster_id", "priority_score", "evidence"}}
	for _, n := range a.Nodes {
		nodes = append(nodes, []string{domain.ID(n.GID), n.Role, number(n.RoleScore), strconv.Itoa(n.ClusterID), number(n.Priority), n.Evidence})
	}
	clusters := [][]string{{"cluster_id", "n_nodes", "n_seed", "sum_kzt_internal", "top_gids", "hypothesis"}}
	for _, c := range a.Clusters {
		clusters = append(clusters, []string{strconv.Itoa(c.ID), strconv.Itoa(c.Nodes), strconv.Itoa(c.Seeds), c.Internal.String(), "[" + strings.Join(c.TopGIDs, ", ") + "]", c.Hypothesis})
	}
	top := [][]string{{"rank", "gid", "role", "priority_score", "why"}}
	for _, n := range a.Top {
		top = append(top, []string{strconv.Itoa(n.Rank), domain.ID(n.GID), n.Role, number(n.Priority), n.Why})
	}
	result := map[string][]byte{}
	for name, rows := range map[string][][]string{"nodes_roles.csv": nodes, "clusters.csv": clusters, "top_nodes.csv": top} {
		var buf bytes.Buffer
		writer := csv.NewWriter(&buf)
		writer.UseCRLF = true
		if err := writer.WriteAll(rows); err != nil {
			return nil, err
		}
		result[name] = buf.Bytes()
	}
	return result, nil
}

// WriteDirectory creates a new result directory so a failed run cannot partially
// replace an earlier export. Rename publishes the completed directory.
func WriteDirectory(dir string, files map[string][]byte) error {
	dir = filepath.Clean(dir)
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		return fmt.Errorf("каталог результатов уже существует или недоступен; укажите новый --output-dir")
	}
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, ".export-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for _, name := range []string{"nodes_roles.csv", "clusters.csv", "top_nodes.csv"} {
		if err := os.WriteFile(filepath.Join(tmp, name), files[name], 0600); err != nil {
			return err
		}
	}
	return os.Rename(tmp, dir)
}
