package domain

import "fmt"

type Gap struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Evidence    string `json:"evidence"`
}
type NextRequest struct {
	Code    string `json:"gap_code"`
	Request string `json:"request"`
	Reason  string `json:"reason"`
}
type Card struct {
	ID          string        `json:"analysis_id"`
	Node        NodeResult    `json:"node"`
	Incoming    []EdgeResult  `json:"incoming"`
	Outgoing    []EdgeResult  `json:"outgoing"`
	Limitations []string      `json:"limitations"`
	Gaps        []Gap         `json:"data_gaps"`
	Requests    []NextRequest `json:"next_requests"`
}

func Limitations(n NodeResult) []string {
	notes := []string{"Выводы относятся только к загруженным переводам; полнота выборки и полный баланс неизвестны", "Роль и оценки — гипотезы для проверки, не вероятность и не вывод о виновности"}
	if n.Seed {
		notes = append(notes, "Для seed полнота входящих неизвестна; отношение out/in не трактуется")
	}
	if n.Truncated {
		notes = append(notes, "depth=4 и отсутствие исходящих — граница наблюдения, не доказательство удержания")
	}
	if n.InDegree+n.OutDegree == 0 {
		notes = append(notes, "Узел изолирован в выборке; отсутствие связей не доказывает отсутствие операций")
	}
	return notes
}
func NodeCard(a Analysis, n NodeResult) Card {
	c := Card{ID: a.ID, Node: n, Incoming: []EdgeResult{}, Outgoing: []EdgeResult{}, Limitations: Limitations(n), Gaps: []Gap{}, Requests: []NextRequest{}}
	for _, e := range a.Edges {
		if e.Dst == n.GID {
			c.Incoming = append(c.Incoming, e)
		}
		if e.Src == n.GID {
			c.Outgoing = append(c.Outgoing, e)
		}
	}
	add := func(code, description, evidence, request, reason string) {
		c.Gaps = append(c.Gaps, Gap{code, description, evidence})
		c.Requests = append(c.Requests, NextRequest{code, request, reason})
	}
	if n.Truncated {
		add("DEPTH_BOUNDARY", "Исходящие за границей обхода неизвестны", fmt.Sprintf("depth=%d; out_deg=%d", n.Depth, n.OutDegree), "Запросить исходящие переводы за пределами обхода за тот же период", "Проверить продолжение движения средств")
	}
	if n.Seed {
		add("SEED_INCOMING_INCOMPLETE", "Полнота входящих seed неизвестна; out/in не является балансом", "is_seed=true", "Запросить полную историю входящих за период выборки", "Проверить источники поступлений")
	}
	if n.InDegree+n.OutDegree == 0 {
		add("ISOLATED_NODE", "Отсутствие связей не доказывает отсутствие деятельности", "in_deg=0; out_deg=0", "Проверить полноту выгрузки и условия отбора", "Выяснить причину отсутствия наблюдаемых связей")
	}
	excess := n.Outgoing.GreaterThan(n.Incoming.Decimal)
	if excess {
		add("OUTFLOW_EXCEEDS_INFLOW", "Выход больше наблюдаемого входа; возможны начальный остаток или отсутствующие поступления", fmt.Sprintf("out_kzt=%s; in_kzt=%s", n.Outgoing.String(), n.Incoming.String()), "Запросить остатки и полную выписку", "Проверить источники покрытия исходящего оборота")
	}
	add("COVERAGE_UNKNOWN", "Банковский охват и пороги отбора не заданы входными файлами", "В схеме нет метаданных об охвате и фильтрах", "Запросить описание источника, охвата банков и фильтров выгрузки", "Установить условия наблюдения без предположений о пропущенных переводах")
	add("LIMITED_PERIOD", "Операции за пределами выгрузки неизвестны", "Даты относятся только к загруженным транзакциям", "Запросить границы полного периода и выписки за соседние периоды", "Проверить устойчивость гипотез")
	if !excess {
		add("BALANCES_UNAVAILABLE", "Переводы не определяют полный баланс счёта", "В схеме нет остатков", "Запросить остатки на начало и конец периода", "Проверить гипотезу об удержании средств")
	}
	return c
}
