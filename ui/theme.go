package ui

import (
	"io/fs"
	"strings"

	"github.com/xmhrnx/giode"
	props "github.com/xmhrnx/giode/styles/properties"

	"github.com/xmhrnx/profilemanager/assets"
)

// Palettes. Dark is the signature look: near-black graphite with a
// violet-to-blue accent; light mirrors it on soft white.
var (
	darkPalette = giode.Palette{
		Primary:    "#8B6CFF",
		Secondary:  "#A4AABC",
		Background: "#0B0C10",
		Surface:    "#13151C",
		Text:       "#ECEEF4",
		Border:     "#252935",
		OnPrimary:  "#FFFFFF",
		Muted:      "#8C93A6",
		Accent:     "#3EA6FF",
		Success:    "#34D399",
		Warning:    "#F5B544",
		Danger:     "#F26D6D",
		Info:       "#5EA8FF",
	}
	lightPalette = giode.Palette{
		Primary:    "#6A4BFF",
		Secondary:  "#5A6175",
		Background: "#F3F4F8",
		Surface:    "#FFFFFF",
		Text:       "#12141B",
		Border:     "#E2E4EC",
		OnPrimary:  "#FFFFFF",
		Muted:      "#697086",
		Accent:     "#1E88E5",
		Success:    "#0F9F6E",
		Warning:    "#C7811A",
		Danger:     "#D64545",
		Info:       "#2A7BE4",
	}
)

// Theme tokens beyond the palette, mixed from it so they follow
// light/dark switches.
var (
	colBackground = giode.Color("theme.background")
	colSurface    = giode.Color("theme.surface")
	colText       = giode.Color("theme.text")
	colMuted      = giode.Color("theme.muted")
	colBorder     = giode.Color("theme.border")
	colPrimary    = giode.Color("theme.primary")
	colAccent     = giode.Color("theme.accent")
	colSuccess    = giode.Color("theme.success")
	colWarning    = giode.Color("theme.warning")
	colDanger     = giode.Color("theme.danger")

	// colSidebar sits between the window and the cards.
	colSidebar = giode.Mix("theme.surface", "theme.background", 0.55)
	// colRaised is a card or a control resting on the surface.
	colRaised = giode.Mix("theme.text", "theme.surface", 0.035)
	// colHover and colPressed shade quiet controls.
	colHover   = giode.Mix("theme.text", "theme.surface", 0.07)
	colPressed = giode.Mix("theme.text", "theme.surface", 0.11)
	// colSubtle is faint text: hints, timestamps.
	colSubtle = giode.Mix("theme.muted", "theme.surface", 0.25)
	// colPrimarySoft tints selected rows and badges.
	colPrimarySoft  = giode.Mix("theme.primary", "theme.surface", 0.14)
	colPrimaryLine  = giode.Mix("theme.primary", "theme.surface", 0.45)
	colSuccessSoft  = giode.Mix("theme.success", "theme.surface", 0.14)
	colWarningSoft  = giode.Mix("theme.warning", "theme.surface", 0.12)
	colDangerSoft   = giode.Mix("theme.danger", "theme.surface", 0.12)
	colInfoSoft     = giode.Mix("theme.accent", "theme.surface", 0.12)
	colBorderStrong = giode.Mix("theme.text", "theme.border", 0.12)
	colShadow       = giode.Color("#00000055")
)

// Fonts.
const (
	fontUI   = "Inter"
	fontMono = "JetBrains Mono"
)

// brandGradient is the accent used for primary actions and the logo.
func brandGradient() *giode.LinearGradient {
	return giode.Linear(135, "#8B5CFF", "#3B82F6")
}

// setupTheme registers fonts and icons and applies the palettes.
func setupTheme() error {
	giode.SetPalette(giode.ModeDark, darkPalette)
	giode.SetPalette(giode.ModeLight, lightPalette)
	if err := registerFonts(); err != nil {
		return err
	}
	// Inter for all text without a FontFamily, Giode's components
	// included; set before the window opens.
	giode.SetDefaultFont(fontUI)
	return giode.RegisterIconsFS(assets.FS, "icons")
}

// registerFonts adds Inter, the interface font, and JetBrains Mono for
// paths and commands.
func registerFonts() error {
	var files []giode.FontFile
	for _, f := range []struct {
		file, family string
		weight       giode.FontWeight
	}{
		{"fonts/Inter-Regular.ttf", fontUI, giode.FontWeightNormal},
		{"fonts/Inter-Medium.ttf", fontUI, giode.FontWeightMedium},
		{"fonts/Inter-SemiBold.ttf", fontUI, giode.FontWeightSemiBold},
		{"fonts/Inter-Bold.ttf", fontUI, giode.FontWeightBold},
		{"fonts/JetBrainsMono-Regular.ttf", fontMono, giode.FontWeightNormal},
	} {
		data, err := fs.ReadFile(assets.FS, f.file)
		if err != nil {
			return err
		}
		files = append(files, giode.FontFile{Family: f.family, Weight: f.weight, Data: data})
	}
	return giode.RegisterFont(files...)
}

// icon renders a Lucide icon.
func icon(name string, size int, color giode.Color) giode.Element {
	return giode.Icon(name, giode.Styles{Width: props.Width(size), Height: props.Height(size), Color: color})
}

// Text helpers in the type scale.

func txt(s string, st giode.Styles) giode.Element {
	if st.Color == "" {
		st.Color = colText
	}
	return giode.Text(s, st)
}

func title(s string, size int) giode.Element {
	return txt(s, giode.Styles{FontSize: props.FontSize(size), FontWeight: giode.FontWeightSemiBold, LineHeight: props.LineHeight(size + size/3)})
}

func body(s string) giode.Element {
	return txt(s, giode.Styles{FontSize: 13, LineHeight: 20})
}

func muted(s string, size int) giode.Element {
	return txt(s, giode.Styles{FontSize: props.FontSize(size), Color: colMuted, LineHeight: props.LineHeight(size + 6)})
}

func mono(s string, size int, color giode.Color) giode.Element {
	return txt(s, giode.Styles{FontFamily: fontMono, FontSize: props.FontSize(size), Color: color, LineHeight: props.LineHeight(size + 7)})
}

// overline is a small uppercase section label.
func overline(s string) giode.Element {
	return txt(s, giode.Styles{FontSize: 11, FontWeight: giode.FontWeightSemiBold, Color: colSubtle, TextTransform: giode.TextTransformUppercase})
}

// plain drops Markdown code marks from definition text.
func plain(s string) string { return strings.ReplaceAll(s, "`", "") }

// spacer grows to take the free space in a row or stack.
func spacer() giode.Element { return giode.Box(giode.Styles{FlexGrow: 1}) }

// inputStyles is the look of text fields.
func inputStyles() giode.Styles {
	return giode.Styles{
		Background: colRaised, Color: colText, BorderColor: colBorder, BorderWidth: 1,
		BorderRadius: 10, Padding: giode.SymmetricInset(12, 9), FontSize: 13,
	}
}

// monoInputStyles is the look of fields holding paths and commands.
func monoInputStyles() giode.Styles {
	st := inputStyles()
	st.FontFamily = fontMono
	st.FontSize = 12
	return st
}

// cardStyles is the base of cards and panels.
func cardStyles() giode.Styles {
	return giode.Styles{
		Background: colSurface, BorderColor: colBorder, BorderWidth: 1, BorderRadius: 14,
		Padding: giode.UniformInset(16),
	}
}
