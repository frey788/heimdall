package config

import "testing"

func TestBuildEmbeddedConfigWithoutPIN(t *testing.T) {
	cfg, err := BuildEmbeddedConfig(InstallerInput{DashboardPath: "heimdall", PINEnabled: false})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Mode != AccessModeEmbedded {
		t.Fatalf("expected mode %s, got %s", AccessModeEmbedded, cfg.Mode)
	}
	if cfg.Path != "/heimdall" {
		t.Fatalf("expected normalized path /heimdall, got %s", cfg.Path)
	}
	if cfg.Protection.Enabled {
		t.Fatal("expected pin protection disabled")
	}
}

func TestBuildEmbeddedConfigWithPIN(t *testing.T) {
	cfg, err := BuildEmbeddedConfig(InstallerInput{DashboardPath: "/inspect", PINEnabled: true, PIN: "1234"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !cfg.Protection.Enabled {
		t.Fatal("expected pin protection enabled")
	}
	if cfg.Protection.PINHash == "" || cfg.Protection.PINSalt == "" {
		t.Fatal("expected stored pin hash and salt")
	}

	ok, err := cfg.Protection.VerifyPIN("1234")
	if err != nil {
		t.Fatalf("expected no verify error, got %v", err)
	}
	if !ok {
		t.Fatal("expected pin verification to pass")
	}

	ok, err = cfg.Protection.VerifyPIN("4321")
	if err != nil {
		t.Fatalf("expected no verify error, got %v", err)
	}
	if ok {
		t.Fatal("expected pin verification to fail")
	}
}

func TestBuildEmbeddedConfigRequiresMinPINLength(t *testing.T) {
	_, err := BuildEmbeddedConfig(InstallerInput{DashboardPath: "/inspect", PINEnabled: true, PIN: "123"})
	if err == nil {
		t.Fatal("expected error for short pin")
	}
}

func TestNormalizeDashboardPathUsesDefault(t *testing.T) {
	path, err := NormalizeDashboardPath("")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if path != DefaultDashboardPath {
		t.Fatalf("expected default path %s, got %s", DefaultDashboardPath, path)
	}
}
