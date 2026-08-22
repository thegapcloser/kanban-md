package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	glamourstyles "github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
)

// intraWordHyphen matches a hyphen between two word characters (inside compound
// words like "chunk-index-eval"), but not markdown syntax like "- list item".
var intraWordHyphen = regexp.MustCompile(`(\w)-(\w)`) //nolint:gochecknoglobals // compiled regex

// nonBreakingHyphen (U+2011) looks identical to a regular hyphen but is not
// treated as a line-break opportunity by word-wrap algorithms.
const nonBreakingHyphen = "\u2011"

// renderMarkdown renders body text as terminal-friendly markdown using glamour.
// Single newlines are preserved as hard line breaks via WithPreservedNewLines.
// Intra-word hyphens are temporarily replaced with non-breaking hyphens to
// prevent glamour's word wrapper from creating short orphan line fragments.
func renderMarkdown(body string, width int) string {
	return renderMarkdownForBackground(body, width, lipgloss.HasDarkBackground())
}

func renderMarkdownForBackground(body string, width int, darkBackground bool) string {
	// Pre-process: protect intra-word hyphens from line breaking.
	body = intraWordHyphen.ReplaceAllString(body, "${1}"+nonBreakingHyphen+"${2}")

	style := glamourstyles.LightStyleConfig
	if darkBackground {
		style = glamourstyles.DarkStyleConfig
	}
	// Keep the document foreground tied to the terminal default instead of
	// baking in the palette's light or dark body color. Terminals update their
	// default foreground when their theme changes, so an already-running TUI
	// stays readable even though Lip Gloss caches its background detection.
	style.Document.Color = nil

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		return lipgloss.NewStyle().Width(width).Render(body)
	}
	out, err := r.Render(body)
	if err != nil {
		return lipgloss.NewStyle().Width(width).Render(body)
	}

	// Post-process: restore regular hyphens.
	out = strings.ReplaceAll(out, nonBreakingHyphen, "-")

	return strings.TrimRight(out, "\n")
}

// unescapeBody replaces literal escape sequences in body text with their
// corresponding whitespace characters. This handles bodies set via CLI flags
// where \n and \t are passed as literal two-character sequences.
func unescapeBody(s string) string {
	r := strings.NewReplacer(
		`\n`, "\n",
		`\t`, "\t",
		`\r`, "",
		`\\`, `\`,
	)
	return r.Replace(s)
}
