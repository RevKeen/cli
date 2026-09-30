package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// RenderTable prints a Lip Gloss table for list/get style payloads.
func RenderTable(data interface{}) error {
	switch v := data.(type) {
	case map[string]interface{}:
		if items, ok := extractDataArray(v); ok {
			return renderRows(items)
		}
		return renderKeyValue(v)
	case []interface{}:
		items := make([]map[string]interface{}, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				items = append(items, m)
			}
		}
		if len(items) == 0 {
			fmt.Println(MutedStyle.Render("No results."))
			return nil
		}
		return renderRows(items)
	default:
		fmt.Printf("%v\n", data)
		return nil
	}
}

func extractDataArray(m map[string]interface{}) ([]map[string]interface{}, bool) {
	raw, ok := m["data"]
	if !ok {
		return nil, false
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil, false
	}
	items := make([]map[string]interface{}, 0, len(arr))
	for _, item := range arr {
		if row, ok := item.(map[string]interface{}); ok {
			items = append(items, row)
		}
	}
	return items, true
}

func renderKeyValue(m map[string]interface{}) error {
	keys := sortedKeys(m)
	var b strings.Builder
	for _, k := range keys {
		_, _ = fmt.Fprintf(&b, "%s  %s\n",
			lipgloss.NewStyle().Foreground(ColorMuted).Width(22).Render(k),
			truncate(fmt.Sprintf("%v", m[k]), 80),
		)
	}
	fmt.Print(BorderStyle.Render(strings.TrimRight(b.String(), "\n")))
	fmt.Println()
	return nil
}

func renderRows(items []map[string]interface{}) error {
	if len(items) == 0 {
		fmt.Println(MutedStyle.Render("No results."))
		return nil
	}

	headers := preferredHeaders(items[0])
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		row := make([]string, len(headers))
		for i, h := range headers {
			row[i] = truncate(fmt.Sprintf("%v", item[h]), 40)
		}
		rows = append(rows, row)
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(ColorBorder)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().Bold(true).Foreground(ColorBrand)
			}
			if row%2 == 0 {
				return lipgloss.NewStyle().Foreground(ColorFg)
			}
			return lipgloss.NewStyle().Foreground(ColorMuted)
		}).
		Headers(headers...).
		Rows(rows...)

	fmt.Println(t.Render())
	return nil
}

func preferredHeaders(sample map[string]interface{}) []string {
	priority := []string{"id", "object", "name", "email", "status", "amount", "currency", "created_at", "updated_at"}
	var headers []string
	seen := map[string]bool{}
	for _, p := range priority {
		if _, ok := sample[p]; ok {
			headers = append(headers, p)
			seen[p] = true
		}
	}
	rest := sortedKeys(sample)
	for _, k := range rest {
		if !seen[k] && len(headers) < 8 {
			headers = append(headers, k)
		}
	}
	return headers
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func truncate(s string, max int) string {
	if max <= 3 || len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
