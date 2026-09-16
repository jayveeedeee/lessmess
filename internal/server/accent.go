package server

// The accent palette is the single source of truth for lessmess's accent
// color assortment: the CSS variables are injected from it (page head),
// the brand assets are rendered from it, and the settings UI reads it via
// /api/settings/options — no color lists are duplicated elsewhere.
//
// Initialization rolls a random entry once: the first resolution that
// finds ui.accent unset in both layers persists a random pick to the
// personal settings layer (.lessmess/settings.json). From then on the
// stored value wins; clearing it in both layers rolls again on the next
// resolution. Unknown stored ids fail open to the default (orange) and
// are never silently rewritten.

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
)

// AccentColor is one palette entry: dark values apply to the dark theme
// (the default), light values to [data-theme="light"]. Hover is the
// --accent-hover companion of each base value.
type AccentColor struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Dark       string `json:"dark"`
	DarkHover  string `json:"darkHover"`
	Light      string `json:"light"`
	LightHover string `json:"lightHover"`
}

// AccentPalette is the curated assortment; the first entry is the
// built-in default. Base values sit in the Tailwind 500 step (600 for
// light hues like amber, so white-on-accent text stays legible in the
// dark theme); light-theme values use the 600/700 step for contrast on
// the light background. Orange carries the historical identity values.
var AccentPalette = []AccentColor{
	{ID: "orange", Label: "Orange", Dark: "#e8641f", DarkHover: "#f47a3a", Light: "#d5550f", LightHover: "#b8480b"},
	{ID: "teal", Label: "Teal", Dark: "#14b8a6", DarkHover: "#2dd4bf", Light: "#0d9488", LightHover: "#0f766e"},
	{ID: "green", Label: "Green", Dark: "#22c55e", DarkHover: "#4ade80", Light: "#16a34a", LightHover: "#15803d"},
	{ID: "blue", Label: "Blue", Dark: "#3b82f6", DarkHover: "#60a5fa", Light: "#2563eb", LightHover: "#1d4ed8"},
	{ID: "violet", Label: "Violet", Dark: "#8b5cf6", DarkHover: "#a78bfa", Light: "#7c3aed", LightHover: "#6d28d9"},
	{ID: "pink", Label: "Pink", Dark: "#ec4899", DarkHover: "#f472b6", Light: "#db2777", LightHover: "#be185d"},
	{ID: "fuchsia", Label: "Fuchsia", Dark: "#d946ef", DarkHover: "#e879f9", Light: "#c026d3", LightHover: "#a21caf"},
	{ID: "red", Label: "Red", Dark: "#ef4444", DarkHover: "#f87171", Light: "#dc2626", LightHover: "#b91c1c"},
	{ID: "amber", Label: "Amber", Dark: "#d97706", DarkHover: "#f59e0b", Light: "#d97706", LightHover: "#b45309"},
	{ID: "cyan", Label: "Cyan", Dark: "#06b6d4", DarkHover: "#22d3ee", Light: "#0891b2", LightHover: "#0e7490"},
}

// DefaultAccent returns the built-in default (the first palette entry).
func DefaultAccent() AccentColor {
	return AccentPalette[0]
}

// AccentByID looks up one palette entry; ok is false for unknown ids
// (including the empty id).
func AccentByID(id string) (AccentColor, bool) {
	for _, a := range AccentPalette {
		if a.ID == id {
			return a, true
		}
	}
	return AccentColor{}, false
}

// accentSession caches a rolled accent per repo dir when persisting the
// roll fails, so the color stays stable for the process lifetime until a
// persist succeeds. Entries are deleted on success so the documented
// "clear both layers to re-roll" semantic stays live.
var accentSession sync.Map // repoDir → AccentColor

// ResolveAccent returns the effective accent for repoDir, rolling and
// persisting a random palette entry on first use (see the package
// comment). It is the read path for every accent consumer.
func ResolveAccent(repoDir string) AccentColor {
	eff, _, loadErr := loadEffectiveSettings(repoDir)
	if loadErr != "" {
		slog.Warn("settings load failed; using defaults", "err", loadErr)
	}
	if id := eff.UI.Accent; id != "" {
		a, ok := AccentByID(id)
		if !ok {
			slog.Warn("unknown ui.accent; using default", "id", id)
			return DefaultAccent()
		}
		return a
	}
	a := AccentPalette[rand.IntN(len(AccentPalette))]
	if err := persistAccent(repoDir, a); err != nil {
		if cached, ok := accentSession.Load(repoDir); ok {
			return cached.(AccentColor)
		}
		accentSession.Store(repoDir, a)
		slog.Warn("accent roll persist failed; using session value", "id", a.ID, "err", err)
		return a
	}
	accentSession.Delete(repoDir)
	slog.Info("rolled initial accent color", "id", a.ID)
	return a
}

// effectiveAccentColor returns the configured accent without rolling —
// the default when unset or invalid. Used where a side effect is
// unwanted, such as the setup shell (no store yet, never rolls).
func effectiveAccentColor(repoDir string) AccentColor {
	eff, _, loadErr := loadEffectiveSettings(repoDir)
	if loadErr != "" {
		slog.Warn("settings load failed; using defaults", "err", loadErr)
	}
	a, ok := AccentByID(eff.UI.Accent)
	if !ok {
		return DefaultAccent()
	}
	return a
}

// persistAccent writes the rolled id to the personal layer, preserving
// any other personal ui.* fields: applySettingsPatch replaces whole
// sections, so the patch must carry the current section plus the accent.
func persistAccent(repoDir string, a AccentColor) error {
	ui := loadSettingsState(repoDir).Personal.UI
	ui.Accent = a.ID
	section, err := json.Marshal(ui)
	if err != nil {
		return err
	}
	body := []byte(`{"ui":` + string(section) + `}`)
	return applySettingsPatch(repoDir, SettingsScopePersonal, body)
}

// validateAccentChoice rejects a submitted accent id that is not in the
// palette. Unlike agent/model validation this is offline and always
// enforced — the palette is static, so there is no service to be down.
func validateAccentChoice(accent string) error {
	if accent == "" {
		return nil
	}
	if _, ok := AccentByID(accent); !ok {
		return fmt.Errorf("unknown accent color %q (not in the palette)", accent)
	}
	return nil
}
