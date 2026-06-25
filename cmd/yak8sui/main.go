package main

import (
	"fmt" // printing to standard output
	"log" // logging that can also stop the program (log.Fatalf)

	"github.com/gdamore/tcell/v2" // Keys and mouse events
	"github.com/rivo/tview"

	// our own package; the import path is "module name" + folder path.
	"yak8sui/pkg/k8s"
)

func main() {
	namespace := "kube-system"

	// 1. Create the application
	app := tview.NewApplication()

	// 2. Create the table for displaying pods
	table := tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false)

	table.SetBorder(true).
		SetTitle(fmt.Sprintf(" Pods in namespace: [%s] ", namespace)).
		SetTitleAlign(tview.AlignLeft)

	// Шапка таблицы
	table.SetCell(0, 0, tview.NewTableCell("#").SetTextColor(tcell.ColorYellow).SetSelectable(false))
	table.SetCell(0, 1, tview.NewTableCell("Pod Name").SetTextColor(tcell.ColorYellow).SetSelectable(false))
	table.SetCell(0, 2, tview.NewTableCell("Status").SetTextColor(tcell.ColorYellow).SetSelectable(false))

	// 3. Function to request data from your k8s package and update the table
	refreshData := func() {
		pods, err := k8s.GetPodNames(namespace)
		if err != nil {
			// If there's an error, display it in red
			table.SetCell(1, 1, tview.NewTableCell(fmt.Sprintf("Error: %v", err)).SetTextColor(tcell.ColorRed))
			return
		}

		// If no pods are found
		if len(pods) == 0 {
			table.SetCell(1, 1, tview.NewTableCell("Pods not found").SetTextColor(tcell.ColorOrange))
			return
		}

		// Clear old data (except the header)
		for r := table.GetRowCount() - 1; r > 0; r-- {
			table.RemoveRow(r)
		}

		// Fill the table with new data
		for i, pod := range pods {
			row := i + 1

			// Highlight the status: Running — green, otherwise — red/orange
			statusColor := tcell.ColorWhite
			if pod.Status == "Running" {
				statusColor = tcell.ColorGreen
			} else {
				statusColor = tcell.ColorRed
			}

			table.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf("%d", row)).SetTextColor(tcell.ColorGray))
			table.SetCell(row, 1, tview.NewTableCell(pod.Name).SetTextColor(tcell.ColorLightCyan))
			table.SetCell(row, 2, tview.NewTableCell(pod.Status).SetTextColor(statusColor))
		}
	}

	// Call the initial data load
	refreshData()

	// 4. Add button control
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyCtrlC:
			app.Stop() // Exit by Esc or Ctrl+C
		}

		if event.Rune() == 'r' || event.Rune() == 'R' {
			refreshData()
		}

		return event
	})

	// Help
	footer := tview.NewTextView().
		SetText(" [Arrows] Navigation | [[yellow]r[-]] refresh | [[red]Esc[-] / [red]Ctrl+C[-]] exit").
		SetDynamicColors(true)

	// Grid to neatly arrange the table and footer
	grid := tview.NewGrid().
		SetRows(0, 1). // Table takes up the entire screen, footer is 1 row high
		SetColumns(0).
		AddItem(table, 0, 0, 1, 1, 0, 0, true).
		AddItem(footer, 1, 0, 1, 1, 0, 0, false)

	// 5. Run
	if err := app.SetRoot(grid, true).EnableMouse(true).Run(); err != nil {
		log.Fatalf("Error of interface: %v", err)
	}
}
