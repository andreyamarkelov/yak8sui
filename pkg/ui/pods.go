package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"yak8sui/pkg/k8s"
)

func newPodsTable(a *App) *tview.Table {
	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)

	table.SetBorder(true).
		SetTitleAlign(tview.AlignCenter)

	table.SetCell(0, 0, tview.NewTableCell("Pod Name").SetTextColor(tcell.ColorYellow).SetSelectable(false))
	table.SetCell(0, 1, tview.NewTableCell("Status").SetTextColor(tcell.ColorYellow).SetSelectable(false))

	refreshData := func() {
		table.SetTitle(fmt.Sprintf(" Pods in namespace: [%s] ", a.Namespace()))

		for r := table.GetRowCount() - 1; r > 0; r-- {
			table.RemoveRow(r)
		}

		pods, err := k8s.ListPods(a.Namespace())
		if err != nil {
			table.SetCell(1, 1, tview.NewTableCell(fmt.Sprintf("Error: %v", err)).SetTextColor(tcell.ColorRed))
			return
		}

		if len(pods) == 0 {
			table.SetCell(1, 1, tview.NewTableCell("Pods not found").SetTextColor(tcell.ColorOrange))
			return
		}

		for i, pod := range pods {
			row := i + 1

			table.SetCell(row, 0, tview.NewTableCell(pod.Name).SetTextColor(tcell.ColorLightCyan))
			table.SetCell(row, 1, tview.NewTableCell(pod.Status).SetTextColor(statusColor(pod.Status)))
		}
	}

	a.register(ScreenPods, refreshData)
	refreshData()

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyCtrlC:
			a.app.Stop()
		}

		if event.Rune() == 'r' || event.Rune() == 'R' {
			refreshData()
		}

		return event
	})

	return table
}
