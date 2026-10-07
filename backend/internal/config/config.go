package config

import (
	"bufio"
	"fmt"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type AI struct {
	Key     string
	Model   string
	BaseURL string
	Timeout time.Duration
}

// String prevents accidental key disclosure when a configuration is logged.
func (a AI) String() string   { return "AI configuration (credentials redacted)" }
func (a AI) GoString() string { return a.String() }
func (a AI) Configured() bool {
	u, err := url.Parse(a.BaseURL)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" &&
		strings.TrimSpace(a.Key) != "" && strings.TrimSpace(a.Model) != "" && a.Timeout >= time.Second && a.Timeout <= 120*time.Second
}
func AIFromEnvironment() AI {
	base := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	timeout := 30.0
	if value, ok := os.LookupEnv("OPENAI_TIMEOUT_SECONDS"); ok {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 1 || parsed > 120 {
			timeout = 0
		} else {
			timeout = parsed
		}
	}
	return AI{Key: strings.TrimSpace(os.Getenv("OPENAI_API_KEY")), Model: strings.TrimSpace(os.Getenv("OPENAI_MODEL")), BaseURL: strings.TrimRight(base, "/"), Timeout: time.Duration(timeout * float64(time.Second))}
}

// LoadEnv reads only the explicitly named file. There is no shell evaluation,
// interpolation or search of parent directories. Existing variables win.
func LoadEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("файл окружения недоступен")
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	line := 0
	pending := map[string]string{}
	for scanner.Scan() {
		line++
		s := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		s = strings.TrimPrefix(s, "export ")
		key, value, ok := strings.Cut(s, "=")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		valid := key != ""
		for i, c := range key {
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || (i > 0 && c >= '0' && c <= '9')) {
				valid = false
			}
		}
		if !ok || !valid {
			return fmt.Errorf("файл окружения: неверный формат строки %d", line)
		}
		if strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'") {
			if len(value) < 2 || value[len(value)-1] != value[0] {
				return fmt.Errorf("файл окружения: неверные кавычки в строке %d", line)
			}
			value = value[1 : len(value)-1]
		} else {
			if idx := strings.Index(value, " #"); idx >= 0 {
				value = strings.TrimSpace(value[:idx])
			}
		}
		pending[key] = value
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("не удалось прочитать файл окружения")
	}
	for key, value := range pending {
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("не удалось задать переменную окружения")
			}
		}
	}
	return nil
}
