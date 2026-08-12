package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	DefaultDashboardPath = "/_heimdall"
	MinPINLength         = 4
)

type AccessMode string

const (
	AccessModeEmbedded AccessMode = "embedded"
)

type PromptPlan struct {
	AskDashboardPath           bool
	AskPINProtection           bool
	AskPINWhenProtectionActive bool
}

func DefaultPromptPlan() PromptPlan {
	return PromptPlan{
		AskDashboardPath:           true,
		AskPINProtection:           true,
		AskPINWhenProtectionActive: true,
	}
}

type InstallerInput struct {
	DashboardPath string
	PINEnabled    bool
	PIN           string
}

type DashboardConfig struct {
	Mode       AccessMode       `json:"mode"`
	Path       string           `json:"path"`
	Protection ProtectionConfig `json:"protection"`
}

type ProtectionConfig struct {
	Enabled bool   `json:"enabled"`
	PINHash string `json:"pin_hash,omitempty"`
	PINSalt string `json:"pin_salt,omitempty"`
}

func BuildEmbeddedConfig(input InstallerInput) (DashboardConfig, error) {
	path, err := NormalizeDashboardPath(input.DashboardPath)
	if err != nil {
		return DashboardConfig{}, err
	}

	protection := ProtectionConfig{Enabled: input.PINEnabled}
	if input.PINEnabled {
		hash, salt, err := hashPIN(input.PIN)
		if err != nil {
			return DashboardConfig{}, err
		}
		protection.PINHash = hash
		protection.PINSalt = salt
	}

	cfg := DashboardConfig{
		Mode:       AccessModeEmbedded,
		Path:       path,
		Protection: protection,
	}

	if err := cfg.Validate(); err != nil {
		return DashboardConfig{}, err
	}

	return cfg, nil
}

func NormalizeDashboardPath(path string) (string, error) {
	normalized := strings.TrimSpace(path)
	if normalized == "" {
		normalized = DefaultDashboardPath
	}

	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}

	if strings.Contains(normalized, " ") {
		return "", errors.New("dashboard path cannot contain spaces")
	}

	return normalized, nil
}

func (c DashboardConfig) Validate() error {
	if c.Mode != AccessModeEmbedded {
		return fmt.Errorf("mode must be %s", AccessModeEmbedded)
	}
	if c.Path == "" || !strings.HasPrefix(c.Path, "/") {
		return errors.New("path must start with /")
	}

	if c.Protection.Enabled {
		if c.Protection.PINHash == "" || c.Protection.PINSalt == "" {
			return errors.New("pin protection is enabled but pin hash is missing")
		}
	}

	return nil
}

func (p ProtectionConfig) VerifyPIN(pin string) (bool, error) {
	if !p.Enabled {
		return true, nil
	}
	if p.PINSalt == "" || p.PINHash == "" {
		return false, errors.New("pin protection is enabled but pin credentials are incomplete")
	}

	actual := digestPIN(p.PINSalt, pin)
	return actual == p.PINHash, nil
}

func hashPIN(pin string) (hash string, salt string, err error) {
	trimmed := strings.TrimSpace(pin)
	if len(trimmed) < MinPINLength {
		return "", "", fmt.Errorf("pin must be at least %d characters", MinPINLength)
	}

	salt, err = newSalt(16)
	if err != nil {
		return "", "", err
	}

	return digestPIN(salt, trimmed), salt, nil
}

func digestPIN(salt, pin string) string {
	sum := sha256.Sum256([]byte(salt + ":" + pin))
	return hex.EncodeToString(sum[:])
}

func newSalt(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
