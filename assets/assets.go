// Package assets embeds the fonts and icons the interface uses.
package assets

import "embed"

// FS holds fonts/ (Inter and JetBrains Mono, SIL OFL) and icons/
// (Lucide, ISC); their licenses sit beside them.
//
//go:embed fonts/*.ttf icons/*.svg logo.png
var FS embed.FS
