package workspace

import (
	"path/filepath"
	"strings"
)

// DetectLanguage identifies a file's language from its name, extension, and
// optional leading content. Unknown text files are reported as "Text".
func DetectLanguage(path, content string) string {
	name := strings.ToLower(filepath.Base(path))
	if language, ok := namedLanguages[name]; ok {
		return language
	}
	if language, ok := extensionLanguages[strings.ToLower(filepath.Ext(name))]; ok {
		return language
	}
	if strings.HasPrefix(content, "#!") {
		line, _, _ := strings.Cut(content, "\n")
		switch {
		case strings.Contains(line, "python"):
			return "Python"
		case strings.Contains(line, "node"), strings.Contains(line, "deno"):
			return "JavaScript"
		case strings.Contains(line, "ruby"):
			return "Ruby"
		case strings.Contains(line, "bash"), strings.Contains(line, "/sh"):
			return "Shell"
		}
	}
	return "Text"
}

var namedLanguages = map[string]string{
	"dockerfile": "Dockerfile", "makefile": "Makefile", "justfile": "Just",
	"go.mod": "Go Module", "go.sum": "Go Module", "go.work": "Go Module",
	"cargo.toml": "TOML", "gemfile": "Ruby", "cmakelists.txt": "CMake",
	".gitignore": "Git Ignore", ".dockerignore": "Docker Ignore",
}

var extensionLanguages = map[string]string{
	".go": "Go", ".mod": "Go Module", ".sum": "Go Module",
	".js": "JavaScript", ".jsx": "JavaScript", ".mjs": "JavaScript", ".cjs": "JavaScript",
	".ts": "TypeScript", ".tsx": "TypeScript", ".mts": "TypeScript", ".cts": "TypeScript",
	".py": "Python", ".pyi": "Python", ".rs": "Rust", ".rb": "Ruby",
	".java": "Java", ".kt": "Kotlin", ".kts": "Kotlin", ".swift": "Swift",
	".c": "C", ".h": "C", ".cc": "C++", ".cpp": "C++", ".hpp": "C++",
	".cs": "C#", ".fs": "F#", ".fsx": "F#", ".php": "PHP",
	".html": "HTML", ".htm": "HTML", ".css": "CSS", ".scss": "SCSS", ".sass": "Sass",
	".vue": "Vue", ".svelte": "Svelte", ".sql": "SQL", ".graphql": "GraphQL",
	".sh": "Shell", ".bash": "Shell", ".zsh": "Shell", ".fish": "Fish",
	".md": "Markdown", ".mdx": "MDX", ".rst": "reStructuredText",
	".json": "JSON", ".jsonc": "JSON", ".yaml": "YAML", ".yml": "YAML",
	".toml": "TOML", ".xml": "XML", ".ini": "INI", ".env": "Environment",
	".proto": "Protocol Buffers", ".tf": "Terraform", ".lua": "Lua", ".ex": "Elixir",
	".exs": "Elixir", ".erl": "Erlang", ".hrl": "Erlang", ".r": "R",
}
