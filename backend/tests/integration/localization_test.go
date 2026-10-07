package integration_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocalizedAPIAndExportPreserveSnapshot(t *testing.T) {
	handler, service, _ := setup(t, nil, nil, 25<<20)
	for locale, want := range map[string]string{"": "Upload three files", "ru": "Сначала загрузите", "kk-KZ": "Алдымен үш файлды"} {
		request := httptest.NewRequest("GET", "/api/analysis", nil)
		request.Header.Set("Accept-Language", locale)
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, request)
		if result.Code != 404 || !strings.Contains(result.Body.String(), want) {
			t.Fatal(locale, result.Code, result.Body.String())
		}
	}
	if result := upload(t, handler, dataset(t), []string{"nodes", "edges", "transactions"}); result.Code != 200 {
		t.Fatal(result.Code, result.Body.String())
	}
	before, _ := service.Current()
	original := before.Analysis.Nodes[0].Evidence
	for _, locale := range []string{"en", "ru", "kk"} {
		for _, path := range []string{"/api/analysis", "/api/nodes/100000000000000001", "/api/exports/nodes_roles.csv"} {
			request := httptest.NewRequest("GET", path, nil)
			request.Header.Set("Accept-Language", locale)
			result := httptest.NewRecorder()
			handler.ServeHTTP(result, request)
			if result.Code != 200 || result.Header().Get("Content-Language") != locale {
				t.Fatal(locale, path, result.Code, result.Body.String())
			}
			if !strings.Contains(result.Body.String(), "100000000000000001") {
				t.Fatal("identifier changed")
			}
			if locale == "en" && strings.Contains(result.Body.String(), "Признаки") {
				t.Fatal("untranslated evidence")
			}
			if path == "/api/analysis" {
				var obj map[string]any
				if err := json.Unmarshal(result.Body.Bytes(), &obj); err != nil {
					t.Fatal(err)
				}
				if obj["analysis_id"] != before.Analysis.ID {
					t.Fatal("analysis changed")
				}
			}
		}
	}
	after, _ := service.Current()
	if after.Analysis.Nodes[0].Evidence != original {
		t.Fatal("localization mutated persisted evidence")
	}
}
