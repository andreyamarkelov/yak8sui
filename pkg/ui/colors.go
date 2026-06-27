package ui

import "github.com/gdamore/tcell/v2"

func statusColor(status string) tcell.Color {
	switch status {
	case "Running", "Succeeded":
		return tcell.ColorGreen
	case "Pending":
		return tcell.ColorYellow
	default:
		return tcell.ColorRed
	}
}
