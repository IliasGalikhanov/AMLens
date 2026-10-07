package application

import (
	"context"
	"encoding/json"
	"finance.local/amlens/internal/domain"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type Model interface {
	Configured() bool
	Answer(context.Context, string, QuestionContext) (ModelAnswer, error)
}
type ModelAnswer struct {
	Answer string   `json:"answer"`
	GIDs   []string `json:"referenced_gids"`
}
type ContextNode struct {
	domain.NodeResult
	Selected        bool   `json:"selected"`
	Reason          string `json:"priority_reason"`
	Rank            *int   `json:"top_rank"`
	InTransactions  int    `json:"incoming_transactions"`
	OutTransactions int    `json:"outgoing_transactions"`
	SeedCount       int    `json:"seed_neighbors"`
}
type QuestionContext struct {
	Nodes       []ContextNode       `json:"nodes"`
	Edges       []domain.EdgeResult `json:"edges"`
	Nearby      int                 `json:"total_nearby_edges"`
	Limitations []string            `json:"limitations"`
}
type Reference struct {
	GID   string   `json:"gid"`
	Facts []string `json:"facts"`
}
type Answer struct {
	ID          string      `json:"analysis_id"`
	Text        string      `json:"answer"`
	References  []Reference `json:"references"`
	Limitations []string    `json:"limitations"`
}

func (s *Service) Ask(ctx context.Context, id, question string, gids []int64) (Answer, error) {
	snap, err := s.Current()
	if err != nil {
		return Answer{}, err
	}
	if snap.Analysis.ID != id {
		return Answer{}, Fail("STALE_ANALYSIS", "Анализ изменился; обновите граф")
	}
	question = strings.TrimSpace(question)
	if utf8.RuneCountInString(question) < 1 || utf8.RuneCountInString(question) > 2000 || len(gids) < 1 || len(gids) > 5 {
		return Answer{}, Fail("INVALID_QUESTION", "Введите вопрос до 2000 символов и выберите 1–5 разных узлов")
	}
	seen := map[int64]bool{}
	nodes := map[int64]domain.NodeResult{}
	for _, n := range snap.Analysis.Nodes {
		nodes[n.GID] = n
	}
	for _, gid := range gids {
		_, exists := nodes[gid]
		if !exists || seen[gid] {
			return Answer{}, Fail("INVALID_QUESTION", "Выбранный gid отсутствует в анализе или повторяется")
		}
		seen[gid] = true
	}
	if !s.AIConfigured() {
		return Answer{}, Fail("AI_UNAVAILABLE", "AI не настроен: задайте ключ и модель OpenAI")
	}
	qc, err := BuildContext(snap.Analysis, gids)
	if err != nil {
		return Answer{}, err
	}
	reply, err := s.model.Answer(ctx, question, qc)
	current, currentErr := s.Current()
	if currentErr != nil || current.Analysis.ID != id {
		return Answer{}, Fail("STALE_ANALYSIS", "Анализ изменился во время ответа; обновите граф")
	}
	if err != nil || validateAnswer(reply, qc) != nil {
		return Answer{}, Fail("AI_UNAVAILABLE", "Не удалось получить корректный ответ AI")
	}
	result := Answer{ID: id, Text: strings.TrimSpace(reply.Answer), References: []Reference{}, Limitations: qc.Limitations}
	for _, gid := range reply.GIDs {
		for _, n := range qc.Nodes {
			if domain.ID(n.GID) == gid {
				result.References = append(result.References, Reference{gid, referenceFacts(n, qc.Edges)})
				break
			}
		}
	}
	return result, nil
}
func BuildContext(a domain.Analysis, gids []int64) (QuestionContext, error) {
	chosen := map[int64]bool{}
	for _, gid := range gids {
		chosen[gid] = true
	}
	byID := map[int64]domain.NodeResult{}
	for _, n := range a.Nodes {
		byID[n.GID] = n
	}
	ranks := map[int64]int{}
	for _, n := range a.Top {
		ranks[n.GID] = n.Rank
	}
	nearby := []domain.EdgeResult{}
	for _, e := range a.Edges {
		if chosen[e.Src] || chosen[e.Dst] {
			nearby = append(nearby, e)
		}
	}
	sort.Slice(nearby, func(i, j int) bool {
		a, b := nearby[i], nearby[j]
		if !a.Amount.Equal(b.Amount.Decimal) {
			return a.Amount.GreaterThan(b.Amount.Decimal)
		}
		if a.Src != b.Src {
			return a.Src < b.Src
		}
		return a.Dst < b.Dst
	})
	candidates := []domain.EdgeResult{}
	for _, gid := range gids {
		for _, e := range nearby {
			if e.Src == gid {
				candidates = append(candidates, e)
				break
			}
		}
	}
	candidates = append(candidates, nearby...)
	neighbors := map[int64]bool{}
	seen := map[domain.Pair]bool{}
	edges := []domain.EdgeResult{}
	for _, e := range candidates {
		p := domain.Pair{Src: e.Src, Dst: e.Dst}
		if seen[p] {
			continue
		}
		added := map[int64]bool{}
		for _, gid := range []int64{e.Src, e.Dst} {
			if !chosen[gid] && !neighbors[gid] {
				added[gid] = true
			}
		}
		if len(neighbors)+len(added) > 5 {
			continue
		}
		for gid := range added {
			neighbors[gid] = true
		}
		seen[p] = true
		edges = append(edges, e)
		if len(edges) == 10 {
			break
		}
	}
	neighborIDs := make([]int64, 0, len(neighbors))
	for gid := range neighbors {
		neighborIDs = append(neighborIDs, gid)
	}
	sort.Slice(neighborIDs, func(i, j int) bool { return neighborIDs[i] < neighborIDs[j] })
	for {
		qc := QuestionContext{Nodes: []ContextNode{}, Edges: edges, Nearby: len(nearby), Limitations: []string{}}
		all := append(append([]int64{}, gids...), neighborIDs...)
		notes := map[string]bool{}
		for _, gid := range all {
			n := byID[gid]
			var rank *int
			if r, ok := ranks[gid]; ok {
				rank = &r
			}
			qc.Nodes = append(qc.Nodes, ContextNode{n, chosen[gid], n.Why, rank, n.InTx, n.OutTx, n.SeedNeighbors})
			for _, note := range domain.Limitations(n) {
				if !notes[note] {
					notes[note] = true
					qc.Limitations = append(qc.Limitations, note)
				}
			}
		}
		if len(edges) < len(nearby) {
			qc.Limitations = append(qc.Limitations, fmt.Sprintf("Связи усечены: показано %d из %d близких рёбер; это не полный список контрагентов", len(edges), len(nearby)))
		}
		qc.Limitations = append(qc.Limitations, "Проверка ссылок не гарантирует истинность всего текста модели; сверяйте объяснение с фактами и карточками")
		encoded, err := json.Marshal(qc)
		if err != nil {
			return QuestionContext{}, err
		}
		if len(encoded) <= 32<<10 {
			return qc, nil
		}
		if len(neighborIDs) == 0 {
			return QuestionContext{}, Fail("INVALID_QUESTION", "Контекст слишком велик: выберите меньше узлов")
		}
		removed := neighborIDs[len(neighborIDs)-1]
		neighborIDs = neighborIDs[:len(neighborIDs)-1]
		kept := []domain.EdgeResult{}
		for _, e := range edges {
			if e.Src != removed && e.Dst != removed {
				kept = append(kept, e)
			}
		}
		edges = kept
	}
}

var explicitGID = regexp.MustCompile(`(?i)\bgid\s*[:=#]?\s*(-?[0-9]+)`)
var numbers = regexp.MustCompile(`-?[0-9]+`)

func validateAnswer(a ModelAnswer, c QuestionContext) error {
	length := utf8.RuneCountInString(strings.TrimSpace(a.Answer))
	if length < 1 || length > 6000 || len(a.GIDs) < 1 || len(a.GIDs) > 10 {
		return fmt.Errorf("invalid answer")
	}
	allowed := map[string]bool{}
	for _, n := range c.Nodes {
		allowed[domain.ID(n.GID)] = true
	}
	refs := map[string]bool{}
	for _, gid := range a.GIDs {
		if !allowed[gid] || refs[gid] {
			return fmt.Errorf("invalid reference")
		}
		refs[gid] = true
	}
	for _, match := range explicitGID.FindAllStringSubmatch(a.Answer, -1) {
		if !refs[match[1]] {
			return fmt.Errorf("unreferenced gid")
		}
	}
	for _, number := range numbers.FindAllString(a.Answer, -1) {
		digits := strings.TrimPrefix(number, "-")
		if len(digits) >= 16 && len(digits) <= 19 && !refs[number] {
			return fmt.Errorf("unreferenced number")
		}
	}
	return nil
}
func referenceFacts(n ContextNode, edges []domain.EdgeResult) []string {
	rank := "не входит"
	if n.Rank != nil {
		rank = fmt.Sprint(*n.Rank)
	}
	facts := []string{"Роль: " + n.Role, fmt.Sprintf("Оценка роли: %g", n.RoleScore), fmt.Sprintf("Входящих контрагентов: %d", n.InDegree), fmt.Sprintf("Исходящих контрагентов: %d", n.OutDegree), "Входящий оборот: " + n.Incoming.String() + " KZT", "Исходящий оборот: " + n.Outgoing.String() + " KZT", fmt.Sprintf("Приоритет: %g", n.Priority), "Место в top-20: " + rank, fmt.Sprintf("Глубина: %d; seed: %t", n.Depth, n.Seed)}
	for _, e := range edges {
		if e.Src == n.GID || e.Dst == n.GID {
			facts = append(facts, fmt.Sprintf("Наблюдаемая связь %d → %d: %s KZT; операций: %d", e.Src, e.Dst, e.Amount.String(), e.Count))
		}
	}
	return facts
}
