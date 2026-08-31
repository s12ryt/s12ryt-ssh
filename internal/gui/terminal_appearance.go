package gui

import (
	"fmt"
	"image/color"
	"regexp"
	"strconv"
	"strings"

	"s12ryt-ssh/internal/remote"

	"gioui.org/font"
)

type terminalFont string

const (
	terminalFontBuiltin terminalFont = "builtin-mono"
	terminalFontSystem  terminalFont = "system-mono"
)

const (
	terminalFontSizeMin float32 = 8
	terminalFontSizeMax float32 = 32
)

const (
	terminalDefaultForeground = "#d7e6e2"
	terminalDefaultBackground = "#101c1b"
)

type terminalAppearance struct {
	Font       terminalFont
	FontSize   float32
	Foreground string
	Background string
}

type terminalAppearanceFormScope string

const (
	terminalAppearanceAccount terminalAppearanceFormScope = "account"
	terminalAppearanceHost    terminalAppearanceFormScope = "host"
)

type terminalAppearanceFormValues struct {
	Scope             terminalAppearanceFormScope
	HostID            string
	Font              string
	FontSize          string
	Foreground        string
	Background        string
	UseAccountDefault bool
}

func (values terminalAppearanceFormValues) input() (remote.SSHTerminalAppearance, error) {
	fontSize, err := strconv.ParseFloat(strings.TrimSpace(values.FontSize), 32)
	if err != nil {
		return remote.SSHTerminalAppearance{}, err
	}
	appearance := normalizeTerminalAppearance(terminalAppearance{
		Font:       terminalFont(strings.TrimSpace(values.Font)),
		FontSize:   float32(fontSize),
		Foreground: strings.TrimSpace(values.Foreground),
		Background: strings.TrimSpace(values.Background),
	})
	if appearance.Font != terminalFont(strings.TrimSpace(values.Font)) ||
		appearance.FontSize != float32(fontSize) ||
		appearance.Foreground != strings.TrimSpace(values.Foreground) ||
		appearance.Background != strings.TrimSpace(values.Background) {
		return remote.SSHTerminalAppearance{}, fmt.Errorf("invalid terminal appearance")
	}
	return terminalAppearanceToRemote(appearance), nil
}

func (values terminalAppearanceFormValues) clearOverride() bool {
	return values.Scope == terminalAppearanceHost && values.UseAccountDefault && values.HostID != ""
}

func terminalAppearanceFromRemote(value remote.SSHTerminalAppearance) terminalAppearance {
	return normalizeTerminalAppearance(terminalAppearance{
		Font:       terminalFont(value.Font),
		FontSize:   value.FontSize,
		Foreground: value.Foreground,
		Background: value.Background,
	})
}

func terminalAppearanceToRemote(value terminalAppearance) remote.SSHTerminalAppearance {
	value = normalizeTerminalAppearance(value)
	return remote.SSHTerminalAppearance{
		Font:       remote.SSHTerminalFont(value.Font),
		FontSize:   value.FontSize,
		Foreground: value.Foreground,
		Background: value.Background,
	}
}

func terminalAppearanceForHost(base terminalAppearance, host remote.SSHHost) terminalAppearance {
	override := host.Settings.TerminalAppearance
	if override == nil {
		return normalizeTerminalAppearance(base)
	}
	return mergeTerminalAppearance(base, &terminalAppearance{
		Font:       terminalFont(override.Font),
		FontSize:   override.FontSize,
		Foreground: override.Foreground,
		Background: override.Background,
	})
}

var terminalHexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func normalizeTerminalAppearance(appearance terminalAppearance) terminalAppearance {
	if appearance.Font != terminalFontSystem && appearance.Font != terminalFontBuiltin {
		appearance.Font = terminalFontBuiltin
	}
	if appearance.FontSize < terminalFontSizeMin {
		appearance.FontSize = terminalFontSizeMin
	} else if appearance.FontSize > terminalFontSizeMax {
		appearance.FontSize = terminalFontSizeMax
	}
	if !terminalHexColor.MatchString(appearance.Foreground) {
		appearance.Foreground = terminalDefaultForeground
	}
	if !terminalHexColor.MatchString(appearance.Background) {
		appearance.Background = terminalDefaultBackground
	}
	return appearance
}

func mergeTerminalAppearance(base terminalAppearance, override *terminalAppearance) terminalAppearance {
	if override == nil {
		return normalizeTerminalAppearance(base)
	}
	merged := base
	if override.Font != "" {
		merged.Font = override.Font
	}
	if override.FontSize != 0 {
		merged.FontSize = override.FontSize
	}
	if override.Foreground != "" {
		merged.Foreground = override.Foreground
	}
	if override.Background != "" {
		merged.Background = override.Background
	}
	return normalizeTerminalAppearance(merged)
}

func terminalAppearanceColor(value string) color.NRGBA {
	if !terminalHexColor.MatchString(value) {
		return color.NRGBA{}
	}
	red, _ := strconv.ParseUint(value[1:3], 16, 8)
	green, _ := strconv.ParseUint(value[3:5], 16, 8)
	blue, _ := strconv.ParseUint(value[5:7], 16, 8)
	return color.NRGBA{R: uint8(red), G: uint8(green), B: uint8(blue), A: 0xff}
}

func terminalCellColors(cell terminalCell, appearance terminalAppearance) (color.NRGBA, color.NRGBA) {
	appearance = normalizeTerminalAppearance(appearance)
	foreground := terminalAppearanceColor(appearance.Foreground)
	background := terminalAppearanceColor(appearance.Background)
	if cell.Foreground == terminalColorRed {
		foreground = colorDanger
	}
	if cell.Background == terminalColorRed {
		background = colorDanger
	}
	if cell.Reverse {
		foreground, background = background, foreground
	}
	return foreground, background
}

func terminalTypeface(face terminalFont) font.Typeface {
	if face == terminalFontSystem {
		return "Consolas"
	}
	return monoTypeface
}
