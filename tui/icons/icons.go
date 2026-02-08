package icons

import (
	"path/filepath"
	"strings"
)

var useNerdFonts bool

// SetNerdFonts enables or disables Nerd Font icons.
func SetNerdFonts(enabled bool) { useNerdFonts = enabled }

// --- Unicode fallback icons ---

var extIcons = map[string]string{
	".js":      "λ",
	".ts":      "λ",
	".jsx":     "λ",
	".tsx":     "λ",
	".svelte":  "◈",
	".go":      "◆",
	".md":      "≡",
	".json":    "⚙",
	".toml":    "⚙",
	".yaml":    "⚙",
	".yml":     "⚙",
	".css":     "◎",
	".scss":    "◎",
	".graphql": "◇",
	".gql":     "◇",
	".html":    "◁",
	".sql":     "▦",
	".sh":      "▸",
	".py":      "◆",
	".rs":      "◆",
}

var dirIcons = map[string]string{
	"src":        "▪",
	"lib":        "▪",
	"components": "▪",
	"routes":     "▪",
	"models":     "▪",
	"resolvers":  "▪",
	"scripts":    "▸",
	"docs":       "≡",
	"test":       "◌",
	"tests":      "◌",
}

// --- Nerd Font v3 icons ---

var nerdExtIcons = map[string]string{
	".go":      "\ue627", //
	".ts":      "\ue628", //
	".js":      "\ue74e", //
	".jsx":     "\ue7ba", //
	".tsx":     "\ue7ba", //
	".py":      "\ue73c", //
	".rs":      "\ue7a8", //
	".svelte":  "\ue697", //
	".md":      "\ue73e", //
	".json":    "\ue60b", //
	".css":     "\ue749", //
	".scss":    "\ue749", //
	".html":    "\ue736", //
	".toml":    "\ue615", //
	".yaml":    "\ue615", //
	".yml":     "\ue615", //
	".graphql": "\ue662", //
	".gql":     "\ue662", //
	".sql":     "\ue706", //
	".sh":      "\ue795", //
	".rb":      "\ue739", //
	".java":    "\ue738", //
	".lua":     "\ue620", //
	".c":       "\ue61e", //
	".cpp":     "\ue61d", //
	".h":       "\ue61e", //
	".vue":     "\ue6a0", //
	".php":     "\ue73d", //
	".swift":   "\ue755", //
	".kt":      "\ue634", //
	".dart":    "\ue798", //
}

var nerdNameIcons = map[string]string{
	"Dockerfile":  "\ue7b0", //
	"Makefile":    "\ue615", //
	".gitignore":  "\ue702", //
	".env":        "\ue615", //
	"go.mod":      "\ue627", //
	"go.sum":      "\ue627", //
	"package.json": "\ue71e", //
}

var nerdDirIcons = map[string]string{
	"src":        "\uf07c", //
	"lib":        "\uf07c", //
	"components": "\uf085", //
	"routes":     "\uf0e8", //
	"test":       "\uf0c3", //
	"tests":      "\uf0c3", //
	"docs":       "\uf02d", //
	"scripts":    "\ue795", //
	"cmd":        "\ue795", //
	"internal":   "\uf023", //
	"pkg":        "\uf07c", //
	"api":        "\uf0e8", //
	"models":     "\uf1c0", //
}

// ForFile returns an icon for a file based on its extension or name.
func ForFile(path string) string {
	base := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(path))

	if useNerdFonts {
		if icon, ok := nerdNameIcons[base]; ok {
			return icon
		}
		if icon, ok := nerdExtIcons[ext]; ok {
			return icon
		}
		return "\uf15b" //  generic file
	}

	if icon, ok := extIcons[ext]; ok {
		return icon
	}
	return "○"
}

// ForDir returns an icon for a directory name.
func ForDir(name string) string {
	lower := strings.ToLower(name)
	if useNerdFonts {
		if icon, ok := nerdDirIcons[lower]; ok {
			return icon
		}
		return "\uf07b" //  generic folder
	}
	if icon, ok := dirIcons[lower]; ok {
		return icon
	}
	return "▪"
}
