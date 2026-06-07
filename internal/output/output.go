package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/revkeen/cli/internal/config"
	"github.com/spf13/cobra"
)

// Format represents the output format.
type Format string

const (
	FormatJSON  Format = "json"
	FormatTable Format = "table"
	FormatYAML  Format = "yaml"
	FormatCSV   Format = "csv"
)

// ResolveFormat determines output format using priority:
// --agent flag -> --json flag -> --table flag -> explicit --output -> config default -> TTY detection
func ResolveFormat(cmd *cobra.Command) Format {
	root := cmd.Root().PersistentFlags()

	if agent, _ := root.GetBool("agent"); agent {
		return FormatJSON
	}

	if jsonFlag, _ := root.GetBool("json"); jsonFlag {
		return FormatJSON
	}

	if tableFlag, _ := root.GetBool("table"); tableFlag {
		return FormatTable
	}

	if root.Changed("output") {
		format, _ := root.GetString("output")
		return Format(format)
	}

	cfg := config.Load()
	if cfg.Settings.DefaultOutput != "" && cfg.Settings.DefaultOutput != "table" {
		return Format(cfg.Settings.DefaultOutput)
	}

	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return FormatTable
	}
	return FormatJSON
}

// IsAgent returns true when --agent flag is set.
func IsAgent(cmd *cobra.Command) bool {
	agent, _ := cmd.Root().PersistentFlags().GetBool("agent")
	return agent
}

// PrintError writes an error. In agent mode, writes JSON to stderr.
// In normal mode, writes human-readable text to stderr.
func PrintError(cmd *cobra.Command, statusCode int, message string) {
	if IsAgent(cmd) {
		errObj := map[string]interface{}{
			"error": map[string]interface{}{
				"message": message,
				"code":    statusCode,
			},
		}
		data, _ := json.Marshal(errObj)
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), string(data))
	} else {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", message)
	}
}

// PrintRaw writes raw bytes to stdout without re-serialization.
func PrintRaw(data []byte) {
	_, _ = os.Stdout.Write(data)
	if len(data) > 0 && data[len(data)-1] != '\n' {
		_, _ = os.Stdout.Write([]byte{'\n'})
	}
}

// Print formats and prints data according to the resolved format.
func Print(cmd *cobra.Command, data interface{}) error {
	format := ResolveFormat(cmd)
	switch format {
	case FormatJSON:
		return printJSON(cmd, data)
	case FormatYAML:
		return printYAML(data)
	case FormatCSV:
		return printCSV(data)
	default:
		return printTable(data)
	}
}

func printJSON(cmd *cobra.Command, data interface{}) error {
	var formatted []byte
	var err error
	if IsAgent(cmd) {
		formatted, err = json.Marshal(data)
	} else {
		formatted, err = json.MarshalIndent(data, "", "  ")
	}
	if err != nil {
		return err
	}
	fmt.Println(string(formatted))
	return nil
}

func printYAML(data interface{}) error {
	// Simple YAML-like output for basic types
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &m); err != nil {
		// Fallback to JSON
		fmt.Println(string(jsonBytes))
		return nil
	}
	for k, v := range m {
		fmt.Printf("%s: %v\n", k, v)
	}
	return nil
}

func printCSV(data interface{}) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Handle array of objects
	var items []map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &items); err != nil {
		// Single object — wrap in array
		var single map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &single); err != nil {
			fmt.Println(string(jsonBytes))
			return nil
		}
		items = []map[string]interface{}{single}
	}

	if len(items) == 0 {
		return nil
	}

	// Collect headers
	var headers []string
	for k := range items[0] {
		headers = append(headers, k)
	}

	w := csv.NewWriter(os.Stdout)
	_ = w.Write(headers)

	for _, item := range items {
		var row []string
		for _, h := range headers {
			row = append(row, fmt.Sprintf("%v", item[h]))
		}
		_ = w.Write(row)
	}

	w.Flush()
	return nil
}

func printTable(data interface{}) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Handle array of objects
	var items []map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &items); err != nil {
		// Single object — print key-value pairs
		var single map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &single); err != nil {
			fmt.Println(string(jsonBytes))
			return nil
		}
		for k, v := range single {
			fmt.Printf("%-20s %v\n", k, v)
		}
		return nil
	}

	if len(items) == 0 {
		fmt.Println("No results.")
		return nil
	}

	// Collect headers and compute column widths
	var headers []string
	widths := make(map[string]int)
	for k := range items[0] {
		headers = append(headers, k)
		widths[k] = len(k)
	}

	for _, item := range items {
		for _, h := range headers {
			val := fmt.Sprintf("%v", item[h])
			if len(val) > widths[h] {
				widths[h] = len(val)
			}
		}
	}

	// Print header
	var headerParts []string
	for _, h := range headers {
		headerParts = append(headerParts, fmt.Sprintf("%-*s", widths[h], strings.ToUpper(h)))
	}
	fmt.Println(strings.Join(headerParts, "  "))

	// Print separator
	var sepParts []string
	for _, h := range headers {
		sepParts = append(sepParts, strings.Repeat("-", widths[h]))
	}
	fmt.Println(strings.Join(sepParts, "  "))

	// Print rows
	for _, item := range items {
		var rowParts []string
		for _, h := range headers {
			val := fmt.Sprintf("%v", item[h])
			rowParts = append(rowParts, fmt.Sprintf("%-*s", widths[h], val))
		}
		fmt.Println(strings.Join(rowParts, "  "))
	}

	return nil
}
