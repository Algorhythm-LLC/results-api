package openapispec

import (
	"strings"
	"testing"
)

func TestOpenAPIYAMLEmbedded(t *testing.T) {
	if len(YAML) < 200 {
		t.Fatalf("embedded spec too short: %d", len(YAML))
	}
	if !strings.Contains(string(YAML), "openapi: 3.1.0") {
		t.Fatal("expected OpenAPI 3.1.0")
	}
}
