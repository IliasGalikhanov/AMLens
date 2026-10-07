package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAIConfigurationAndRedaction(t *testing.T) {
	base := AI{Key: "unit-test-only-credential", Model: "test-model", BaseURL: "https://api.openai.com/v1", Timeout: 30 * time.Second}
	if !base.Configured() {
		t.Fatal("valid configuration rejected")
	}
	for _, url := range []string{"http://example.test/v1", "https://user:password@example.test/v1", "https://example.test/v1?key=x", "https://example.test/#secret", "://bad"} {
		c := base
		c.BaseURL = url
		if c.Configured() {
			t.Fatal("bad URL accepted")
		}
	}
	for _, timeout := range []time.Duration{0, 121 * time.Second} {
		c := base
		c.Timeout = timeout
		if c.Configured() {
			t.Fatal("invalid timeout accepted")
		}
	}
	if strings.Contains(fmt.Sprintf("%v %+v %#v", base, base, base), base.Key) {
		t.Fatal("configuration leaks secret")
	}
	t.Setenv("OPENAI_TIMEOUT_SECONDS", "NaN")
	if AIFromEnvironment().Configured() {
		t.Fatal("NaN accepted")
	}
}
func TestExplicitEnvIsLiteralAndEnvironmentWins(t *testing.T) {
	t.Setenv("FINANCE_ENV_EXISTING", "environment")
	os.Unsetenv("FINANCE_ENV_NEW")
	defer os.Unsetenv("FINANCE_ENV_NEW")
	path := filepath.Join(t.TempDir(), ".env")
	contents := "FINANCE_ENV_EXISTING=file\nFINANCE_ENV_NEW='$(not-a-command)'\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	if err := LoadEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("FINANCE_ENV_EXISTING") != "environment" || os.Getenv("FINANCE_ENV_NEW") != "$(not-a-command)" {
		t.Fatal("dotenv semantics changed")
	}
	if err := os.WriteFile(path, []byte("this is not valid"), 0600); err != nil {
		t.Fatal(err)
	}
	if LoadEnv(path) == nil {
		t.Fatal("invalid env accepted")
	}
}
