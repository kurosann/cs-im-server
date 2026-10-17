package system

import "testing"

func TestSystemModuleDoesNotRegisterHealthRoutes(t *testing.T) {
	handlers := NewSystemModule().HttpApi().Handlers
	registered := make(map[string]bool)
	for _, handler := range handlers {
		registered[handler.Method+" "+handler.Path] = true
	}
	for _, path := range []string{"GET /healthz", "GET /readyz"} {
		if registered[path] {
			t.Fatalf("%s should use built-in telemetry, not be registered", path)
		}
	}
}
