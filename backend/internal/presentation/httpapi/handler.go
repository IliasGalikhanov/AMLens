package httpapi

import (
	"encoding/json"
	"errors"
	"finance.local/amlens/internal/application"
	"finance.local/amlens/internal/i18n"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type localizedWriter struct {
	http.ResponseWriter
	locale string
}

type Handler struct {
	service  *application.Service
	origins  map[string]bool
	routes   *http.ServeMux
	readOnly bool
}

func New(service *application.Service, origins []string, demo ...bool) http.Handler {
	h := &Handler{service: service, origins: map[string]bool{}, routes: http.NewServeMux()}
	h.readOnly = len(demo) > 0 && demo[0]
	for _, origin := range origins {
		if origin = strings.TrimSpace(origin); origin != "" {
			h.origins[origin] = true
		}
	}
	h.routes.HandleFunc("GET /api/health", h.health)
	h.routes.HandleFunc("GET /api/analysis", h.analysis)
	h.routes.HandleFunc("GET /api/analyses", h.history)
	h.routes.HandleFunc("POST /api/analyses/{id}/activate", h.activate)
	h.routes.HandleFunc("POST /api/analyze", h.analyze)
	h.routes.HandleFunc("GET /api/nodes/{gid}", h.node)
	h.routes.HandleFunc("GET /api/exports/{name}", h.export)
	h.routes.HandleFunc("POST /api/ask", h.ask)
	h.routes.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, 404, "NOT_FOUND", "Маршрут не найден")
	})
	return h
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	locale := i18n.Parse(r.Header.Get("Accept-Language"))
	w = localizedWriter{w, locale}
	r = r.WithContext(i18n.WithLocale(r.Context(), locale))
	w.Header().Set("Content-Language", locale)
	w.Header().Add("Vary", "Accept-Language")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Add("Vary", "Origin")
	if origin := r.Header.Get("Origin"); origin != "" && h.origins[origin] {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Expose-Headers", "X-Analysis-Id, Content-Disposition")
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(204)
			return
		}
	}
	defer func() {
		if recover() != nil {
			writeError(w, 500, "INTERNAL_ERROR", "Не удалось обработать запрос")
		}
	}()
	if h.readOnly && r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, 403, "DEMO_READ_ONLY", "Демонстрация доступна только для просмотра синтетических данных")
		return
	}
	h.routes.ServeHTTP(w, r)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	data, err := json.Marshal(value)
	if err == nil {
		if writer, ok := w.(localizedWriter); ok {
			data, err = i18n.JSON(data, writer.locale)
		}
	}
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Не удалось подготовить ответ")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "details": map[string]any{}}})
}
func respondError(w http.ResponseWriter, err error) {
	var appErr *application.Error
	if errors.As(err, &appErr) {
		statuses := map[string]int{"ANALYSIS_NOT_FOUND": 404, "NO_ANALYSIS": 404, "GID_NOT_FOUND": 404, "EXPORT_NOT_FOUND": 404, "INVALID_SCHEMA": 422, "FILE_TOO_LARGE": 413, "ANALYSIS_BUSY": 409, "STALE_ANALYSIS": 409, "INVALID_QUESTION": 422, "AI_UNAVAILABLE": 503}
		status := statuses[appErr.Code]
		if status == 0 {
			status = 500
		}
		writeError(w, status, appErr.Code, appErr.Message)
		return
	}
	writeError(w, 500, "INTERNAL_ERROR", "Не удалось обработать запрос")
}
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	_, err := h.service.Current()
	writeJSON(w, 200, map[string]any{"status": "ok", "analysis_ready": err == nil, "ai_configured": h.service.AIConfigured(), "demo_mode": h.readOnly})
}
func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	records, err := h.service.History(r.Context())
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, 200, records)
}
func (h *Handler) activate(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.service.Activate(r.Context(), r.PathValue("id"))
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, 200, snapshot.Analysis)
}
func (h *Handler) analysis(w http.ResponseWriter, r *http.Request) {
	snap, err := h.service.Current()
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, 200, snap.Analysis)
}

type multipartSource struct {
	reader *multipart.Reader
	body   io.Reader
	done   bool
}

func (s *multipartSource) Next() (application.Upload, error) {
	if s.done {
		return application.Upload{}, io.EOF
	}
	part, err := s.reader.NextPart()
	if err == io.EOF {
		s.done = true
		_, drainErr := io.Copy(io.Discard, s.body)
		if drainErr != nil {
			return application.Upload{}, drainErr
		}
	}
	if err != nil {
		return application.Upload{}, err
	}
	return application.Upload{Name: part.FormName(), Filename: part.FileName(), Reader: part}, nil
}
func (h *Handler) analyze(w http.ResponseWriter, r *http.Request) {
	const limit = 76 << 20
	if r.ContentLength > limit {
		writeError(w, 413, "FILE_TOO_LARGE", "Размер запроса превышает 76 MiB")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	defer r.Body.Close()
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, 422, "INVALID_SCHEMA", "Ожидается multipart/form-data с тремя Parquet-файлами")
		return
	}
	snap, err := h.service.Publish(r.Context(), &multipartSource{reader: reader, body: r.Body})
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"analysis_id": snap.Analysis.ID, "status": "ready", "summary": snap.Analysis.Summary, "analysis_url": "/api/analysis"})
}

var gidPattern = regexp.MustCompile(`^-?[0-9]{1,19}$`)

func parseGID(s string) (int64, error) {
	if !gidPattern.MatchString(s) {
		return 0, errors.New("invalid gid")
	}
	return strconv.ParseInt(s, 10, 64)
}
func (h *Handler) node(w http.ResponseWriter, r *http.Request) {
	gid, err := parseGID(r.PathValue("gid"))
	if err != nil {
		writeError(w, 422, "INVALID_SCHEMA", "gid должен быть десятичной строкой int64")
		return
	}
	card, err := h.service.Card(gid)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, 200, card)
}
func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	data, id, err := h.service.Export(name)
	if err != nil {
		respondError(w, err)
		return
	}
	data, err = i18n.CSV(data, i18n.Locale(r.Context()))
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Не удалось подготовить ответ")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	w.Header().Set("X-Analysis-Id", id)
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
func (h *Handler) ask(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	defer r.Body.Close()
	var request struct {
		ID       string   `json:"analysis_id"`
		Question string   `json:"question"`
		GIDs     []string `json:"context_gids"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF || strings.TrimSpace(request.ID) == "" || utf8.RuneCountInString(request.ID) > 128 {
		writeError(w, 422, "INVALID_QUESTION", "Проверьте analysis_id, question и context_gids")
		return
	}
	gids := make([]int64, 0, len(request.GIDs))
	for _, value := range request.GIDs {
		gid, err := parseGID(value)
		if err != nil {
			writeError(w, 422, "INVALID_QUESTION", "gid должен быть десятичной строкой int64")
			return
		}
		gids = append(gids, gid)
	}
	answer, err := h.service.Ask(r.Context(), request.ID, request.Question, gids)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, 200, answer)
}
