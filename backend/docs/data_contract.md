# HTTP-контракт AMLens (Go)

API по умолчанию http://127.0.0.1:8000. Во всех HTTP JSON gid/src/dst и clusters.top_gids — **десятичные строки int64**. Не преобразовывайте их в JavaScript Number. Счётчики, глубина, rank и cluster_id — числа; суммы — числа KZT для отображения.

## Маршруты

| Метод | Путь | Ответ |
|---|---|---|
| GET | /api/health | status: "ok", analysis_ready: boolean, ai_configured: boolean, demo_mode: boolean |
| POST | /api/analyze | analysis_id, status: "ready", summary, analysis_url: "/api/analysis" |
| GET | /api/analysis | analysis_id, summary, nodes, edges, clusters, top_nodes |
| GET | /api/analyses | последние 100 записей: analysis_id, created_at (UTC ISO 8601), n_nodes, n_edges |
| POST | /api/analyses/{id}/activate | полный снимок выбранного анализа; выбор сохраняется в SQLite |
| GET | /api/nodes/{gid} | analysis_id, node, incoming, outgoing, limitations, data_gaps, next_requests |
| GET | /api/exports/{name} | CSV; имена nodes_roles.csv, clusters.csv, top_nodes.csv |
| POST | /api/ask | analysis_id, answer, references, limitations |

ai_configured сообщает о настройке ключа, модели, URL и таймаута, а не о доступности сервиса.

При demo_mode=true доступны только GET/HEAD. Запросы записи дают 403 DEMO_READ_ONLY, в том числе /api/analyze, /api/ask и activate. Демо не вызывает внешнюю модель.

## Загрузка

multipart/form-data, ровно три файловых поля nodes, edges, transactions. У каждого имя файла заканчивается на .parquet. Дубликаты полей, дополнительные поля, отсутствующие файлы и неверное содержимое отклоняются. Клиентские имена не используются как пути.

25 MiB на файл и 76 MiB на весь body, проверка фактических байтов. Дополнительно ограничены число строк и распакованный размер: см. [README](../../README.md). Схемы и правила — [input-schema.json](../../docs/input-schema.json).

Ответ приходит после расчёта, подготовки всех CSV и транзакции SQLite. Новый analysis_id публикуется атомарно. Ошибка сохраняет прежний снимок; параллельный расчёт или активация дают 409 ANALYSIS_BUSY. GET продолжает читать предыдущий снимок. После перезапуска восстанавливается активная версия.

## Поля результата

- summary: n_nodes, n_edges, n_transactions, n_seed, n_clusters, edge_volume_kzt.
- node: gid, depth, is_seed, role, role_score, cluster_id, priority_score, evidence, in_deg, out_deg, in_kzt, out_kzt, truncated_by_depth.
- edge: src, dst, sum_kzt, n_tx.
- cluster: cluster_id, n_nodes, n_seed, sum_kzt_internal, top_gids, hypothesis.
- top_node: rank, gid, role, priority_score, why.

Роли: consolidator, transit, distributor, terminal, coordinator, peripheral. Баллы в [0,1]. Массивы всегда JSON-массивы, включая пустые []. Все узлы и направленные рёбра включены, пагинации нет.

Карточка содержит incoming (dst=gid), outgoing (src=gid), limitations (строки). data_gaps: объекты code/description/evidence; next_requests: gap_code/request/reason, один к одному в том же порядке. Коды: DEPTH_BOUNDARY, SEED_INCOMING_INCOMPLETE, ISOLATED_NODE, OUTFLOW_EXCEEDS_INFLOW, COVERAGE_UNKNOWN, LIMITED_PERIOD, BALANCES_UNAVAILABLE. Наличие пропущенных операций не утверждается.

## Экспорт

Content-Type: text/csv; charset=utf-8. Content-Disposition содержит имя файла. X-Analysis-Id содержит версию снимка; frontend обязан сверять её с открытым графом. CORS разрешает чтение этих заголовков для настроенных origins.

CSV-схемы сохранены; gid записан точно. В clusters.csv top_gids — JSON-массив целых, при чтении JavaScript сохраняйте их как строки без преобразования через Number.

## AI-вопрос

Пример формы запроса (идентификатор 1 условный; нужен реальный выбранный узел собственного анализа):

```json
{
  "analysis_id": "идентификатор-из-api-analysis",
  "question": "Почему этот узел получил такой приоритет?",
  "context_gids": ["1"]
}
```

analysis_id — непустая строка до 128 символов; question после обрезки пробелов — 1–2000 символов; context_gids — 1–5 уникальных строк int64, существующих в текущем анализе. Числовые gid и неизвестные поля запроса отклоняются.

answer — текст; references — массив объектов gid и facts (строки из расчёта сервера); limitations — строки. Ответ отображается как текст, не HTML. Если анализ сменился до или во время ответа, 409 STALE_ANALYSIS. Без настройки AI — 503, остальной API работает.

## Ошибки

```json
{"error":{"code":"NO_ANALYSIS","message":"Сначала загрузите три файла и выполните анализ","details":{}}}
```

| HTTP | Коды |
|---|---|
| 403 | DEMO_READ_ONLY |
| 404 | NO_ANALYSIS, ANALYSIS_NOT_FOUND, GID_NOT_FOUND, EXPORT_NOT_FOUND, NOT_FOUND |
| 409 | ANALYSIS_BUSY, STALE_ANALYSIS |
| 413 | FILE_TOO_LARGE |
| 422 | INVALID_SCHEMA, INVALID_QUESTION |
| 503 | AI_UNAVAILABLE |
| 500 | INTERNAL_ERROR |

Валидация указывает файл, поле и строку, когда они известны; сервер не возвращает секреты или сырые ошибки AI. Cache-Control: no-store. Авторизация не реализована; сервер предназначен для локальной работы.
