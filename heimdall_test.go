package heimdall

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInstallDefaults(t *testing.T) {
	runtime, err := Install(InstallOptions{})
	if err != nil {
		t.Fatalf("expected install to succeed, got %v", err)
	}

	if runtime.Config.Path != "/_heimdall" {
		t.Fatalf("expected default dashboard path /_heimdall, got %s", runtime.Config.Path)
	}
	if runtime.Wiring == nil || runtime.Store == nil || runtime.Dashboard == nil {
		t.Fatal("expected runtime wiring, store, and dashboard to be initialized")
	}
}

func TestMountWithPINProtection(t *testing.T) {
	runtime, err := Install(InstallOptions{
		DashboardPath: "inspect",
		PINEnabled:    true,
		PIN:           "1234",
	})
	if err != nil {
		t.Fatalf("expected install to succeed, got %v", err)
	}

	mux := http.NewServeMux()
	if err := runtime.Mount(mux); err != nil {
		t.Fatalf("expected mount to succeed, got %v", err)
	}

	unauthorized := httptest.NewRecorder()
	mux.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/inspect/health", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 without pin, got %d", unauthorized.Code)
	}

	authorized := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/inspect/health", nil)
	req.Header.Set("X-Heimdall-PIN", "1234")
	mux.ServeHTTP(authorized, req)
	if authorized.Code != http.StatusOK {
		t.Fatalf("expected status 200 with pin, got %d", authorized.Code)
	}
}
