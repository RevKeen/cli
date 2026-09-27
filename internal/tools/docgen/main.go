// Package main generates CLI reference documentation from the Cobra command
// tree. Output formats: markdown (default), man pages, or ReST.
// Also produces llms.txt and llms-full.txt for LLM-friendly consumption.
//
// Usage:
//
//	go run ./internal/tools/docgen -out ./docs/cli -format markdown
//	go run ./internal/tools/docgen -out ./docs/cli -format man
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/revkeen/cli/cmd"
	"github.com/spf13/cobra"
	cobradoc "github.com/spf13/cobra/doc"
)

func main() {
	outDir := flag.String("out", "./docs/cli", "Output directory")
	format := flag.String("format", "markdown", "Output format: markdown, man, rest")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", *outDir, err)
	}

	root := cmd.Root()

	var err error
	switch *format {
	case "markdown":
		err = cobradoc.GenMarkdownTree(root, *outDir)
	case "man":
		header := &cobradoc.GenManHeader{
			Title:   "REVKEEN",
			Section: "1",
			Source:  "RevKeen CLI",
		}
		err = cobradoc.GenManTree(root, header, *outDir)
	case "rest":
		err = cobradoc.GenReSTTree(root, *outDir)
	default:
		log.Fatalf("unknown format %q (markdown, man, rest)", *format)
	}

	if err != nil {
		log.Fatalf("docgen: %v", err)
	}

	fmt.Printf("Generated %s docs in %s\n", *format, *outDir)

	// Generate llms.txt and llms-full.txt alongside the markdown docs
	if *format == "markdown" {
		if err := genLLMsTxt(root, *outDir); err != nil {
			log.Fatalf("llms.txt: %v", err)
		}
		fmt.Printf("Generated llms.txt and llms-full.txt in %s\n", *outDir)
	}
}

// genLLMsTxt produces two files:
//   - llms.txt: compact command index with one-line descriptions
//   - llms-full.txt: full concatenated markdown docs for all commands
func genLLMsTxt(root *cobra.Command, outDir string) error {
	// Collect all commands
	var commands []*cobra.Command
	collectCommands(root, &commands)

	// ── llms.txt ──────────────────────────────────────────────────
	var idx bytes.Buffer
	idx.WriteString("# RevKeen CLI\n\n")
	idx.WriteString("> " + root.Short + "\n\n")
	idx.WriteString("## Commands\n\n")

	sort.Slice(commands, func(i, j int) bool {
		return commands[i].CommandPath() < commands[j].CommandPath()
	})

	for _, c := range commands {
		if !c.IsAvailableCommand() && c.Name() != "help" {
			continue
		}
		fmt.Fprintf(&idx, "- `%s` — %s\n", c.CommandPath(), c.Short)
	}

	idx.WriteString("\n## Global Flags\n\n")
	idx.WriteString("```\n")
	idx.WriteString(root.PersistentFlags().FlagUsages())
	idx.WriteString("```\n")

	idx.WriteString("\n## Install\n\n")
	idx.WriteString("```bash\nnpm install -g @revkeen/cli-binary\nbrew install revkeen/tap/revkeen\ncurl -fsSL https://cli.revkeen.com/install.sh | sh\n```\n")

	idx.WriteString("\n## Links\n\n")
	idx.WriteString("- [API Reference](https://docs.revkeen.com/api-reference/openapi)\n")
	idx.WriteString("- [Full CLI docs](https://github.com/revkeen/cli)\n")
	idx.WriteString("- [llms-full.txt](llms-full.txt)\n")

	if err := os.WriteFile(filepath.Join(outDir, "llms.txt"), idx.Bytes(), 0o644); err != nil {
		return err
	}

	// ── llms-full.txt ─────────────────────────────────────────────
	var full bytes.Buffer
	full.WriteString("# RevKeen CLI — Full Reference\n\n")

	for _, c := range commands {
		if !c.IsAvailableCommand() && c.Name() != "help" {
			continue
		}
		var buf bytes.Buffer
		if err := cobradoc.GenMarkdown(c, &buf); err != nil {
			return fmt.Errorf("gen markdown for %s: %w", c.CommandPath(), err)
		}
		// Strip the SEE ALSO section to reduce noise
		content := buf.String()
		if idx := strings.Index(content, "### SEE ALSO"); idx > 0 {
			content = content[:idx]
		}
		full.WriteString(content)
		full.WriteString("\n---\n\n")
	}

	return os.WriteFile(filepath.Join(outDir, "llms-full.txt"), full.Bytes(), 0o644)
}

func collectCommands(cmd *cobra.Command, out *[]*cobra.Command) {
	*out = append(*out, cmd)
	for _, c := range cmd.Commands() {
		collectCommands(c, out)
	}
}
