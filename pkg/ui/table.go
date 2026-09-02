package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PrintTable renders an aligned CLI table taking ANSI color escape codes into account.
func PrintTable(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	cols := len(headers)
	widths := make([]int, cols)

	// 1. Calculate max visible printable width per column
	for i, h := range headers {
		if w := lipgloss.Width(h); w > widths[i] {
			widths[i] = w
		}
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < cols {
				if w := lipgloss.Width(cell); w > widths[i] {
					widths[i] = w
				}
			}
		}
	}

	// 2. Print styled headers
	for i, h := range headers {
		w := lipgloss.Width(h)
		fmt.Print(HeaderStyle.Render(h))
		if i < cols-1 {
			fmt.Print(strings.Repeat(" ", widths[i]-w+3))
		}
	}
	fmt.Println()

	// 3. Print rows with exact visual padding
	for _, row := range rows {
		for i, cell := range row {
			w := lipgloss.Width(cell)
			fmt.Print(cell)
			if i < cols-1 {
				fmt.Print(strings.Repeat(" ", widths[i]-w+3))
			}
		}
		fmt.Println()
	}
}
