package gui

import (
	"image/color"
	"testing"

	"s12ryt-ssh/internal/remote"
)

func TestTerminalAppearanceFormValidatesAndBuildsAccountOrHostUpdate(t *testing.T) {
	account := terminalAppearanceFormValues{
		Scope:      terminalAppearanceAccount,
		Font:       string(terminalFontSystem),
		FontSize:   "17",
		Foreground: " #f0f0f0 ",
		Background: "#080808",
	}
	input, err := account.input()
	if err != nil {
		t.Fatalf("account form input: %v", err)
	}
	if input.Font != remote.SSHTerminalFontSystem || input.FontSize != 17 || input.Foreground != "#f0f0f0" {
		t.Fatalf("account input = %+v", input)
	}

	host := account
	host.Scope = terminalAppearanceHost
	host.HostID = "host-1"
	host.UseAccountDefault = true
	if _, err := host.input(); err != nil {
		t.Fatalf("host inherit input: %v", err)
	}
	if !host.clearOverride() {
		t.Fatal("host default selection must clear the override")
	}
}

func TestTerminalAppearanceFormRejectsInvalidValues(t *testing.T) {
	_, err := (terminalAppearanceFormValues{
		Scope:      terminalAppearanceAccount,
		Font:       "unknown",
		FontSize:   "not-a-number",
		Foreground: "invalid",
		Background: "#000000",
	}).input()
	if err == nil {
		t.Fatal("invalid appearance must be rejected")
	}
}

func TestNormalizeTerminalAppearanceClampsFontAndKeepsPalette(t *testing.T) {
	appearance := normalizeTerminalAppearance(terminalAppearance{
		Font:       terminalFontSystem,
		FontSize:   2,
		Foreground: "#f0f0f0",
		Background: "#101010",
	})
	if appearance.FontSize != terminalFontSizeMin {
		t.Fatalf("font size = %v, want minimum %v", appearance.FontSize, terminalFontSizeMin)
	}
	if appearance.Font != terminalFontSystem {
		t.Fatalf("font = %q, want system", appearance.Font)
	}
	if appearance.Foreground != "#f0f0f0" || appearance.Background != "#101010" {
		t.Fatalf("palette changed: %#v", appearance)
	}
}

func TestTerminalAppearanceRemoteMappingAndHostInheritance(t *testing.T) {
	base := terminalAppearanceFromRemote(remote.SSHTerminalAppearance{
		Font:       remote.SSHTerminalFontSystem,
		FontSize:   15,
		Foreground: "#eeeeee",
		Background: "#050505",
	})
	if got := terminalAppearanceToRemote(base); got.Font != remote.SSHTerminalFontSystem || got.FontSize != 15 {
		t.Fatalf("remote appearance round trip = %+v", got)
	}
	host := remote.SSHHost{Settings: remote.SSHConnectionSettings{
		TerminalAppearance: &remote.SSHTerminalAppearanceOverride{
			FontSize:   19,
			Foreground: "#ffcc00",
		},
	}}
	merged := terminalAppearanceForHost(base, host)
	if merged.Font != terminalFontSystem || merged.FontSize != 19 || merged.Foreground != "#ffcc00" || merged.Background != "#050505" {
		t.Fatalf("merged remote host appearance = %+v", merged)
	}

	host.Settings.TerminalAppearance = nil
	if inherited := terminalAppearanceForHost(base, host); inherited != base {
		t.Fatalf("inherited appearance = %+v, want %+v", inherited, base)
	}
}

func TestNormalizeTerminalAppearanceUsesDefaultsForUnknownValues(t *testing.T) {
	appearance := normalizeTerminalAppearance(terminalAppearance{
		Font:       "unknown",
		FontSize:   200,
		Foreground: "bad",
		Background: "",
	})
	if appearance.Font != terminalFontBuiltin {
		t.Fatalf("font = %q, want builtin", appearance.Font)
	}
	if appearance.FontSize != terminalFontSizeMax {
		t.Fatalf("font size = %v, want maximum %v", appearance.FontSize, terminalFontSizeMax)
	}
	if appearance.Foreground != terminalDefaultForeground || appearance.Background != terminalDefaultBackground {
		t.Fatalf("defaults not applied: %#v", appearance)
	}
}

func TestTerminalAppearanceHostOverrideTakesPrecedence(t *testing.T) {
	base := terminalAppearance{Font: terminalFontBuiltin, FontSize: 14, Foreground: "#aaaaaa", Background: "#111111"}
	override := terminalAppearance{FontSize: 18, Foreground: "#bbbbbb"}
	merged := mergeTerminalAppearance(base, &override)
	if merged.FontSize != 18 || merged.Foreground != "#bbbbbb" || merged.Background != "#111111" {
		t.Fatalf("merged appearance = %#v", merged)
	}
}

func TestTerminalAppearanceColorParsesHexAndFallsBack(t *testing.T) {
	if got := terminalAppearanceColor("#102030"); got != (color.NRGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xff}) {
		t.Fatalf("parsed color = %#v", got)
	}
	if got := terminalAppearanceColor("invalid"); got != (color.NRGBA{R: 0, G: 0, B: 0, A: 0}) {
		t.Fatalf("invalid color = %#v, want transparent", got)
	}
}

func TestTerminalCellColorsApplyAppearanceAndReverse(t *testing.T) {
	appearance := normalizeTerminalAppearance(terminalAppearance{
		Font:       terminalFontBuiltin,
		FontSize:   14,
		Foreground: "#102030",
		Background: "#405060",
	})
	foreground, background := terminalCellColors(terminalCell{}, appearance)
	if foreground != terminalAppearanceColor("#102030") || background != terminalAppearanceColor("#405060") {
		t.Fatalf("cell colors = %#v/%#v", foreground, background)
	}
	foreground, background = terminalCellColors(terminalCell{Reverse: true}, appearance)
	if foreground != terminalAppearanceColor("#405060") || background != terminalAppearanceColor("#102030") {
		t.Fatalf("reverse colors = %#v/%#v", foreground, background)
	}
}
