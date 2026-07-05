package ui

import (
	"fmt"
	"yak8sui/pkg/k8s"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func newDeploymentsTable(a *App) *tview.Table {
	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)

	table.SetBorder(true).
		SetTitleAlign(tview.AlignCenter)

	table.SetCell(0, 0, tview.NewTableCell("Name").SetTextColor(tcell.ColorYellow).SetSelectable(false))
	table.SetCell(0, 1, tview.NewTableCell("Ready").SetTextColor(tcell.ColorYellow).SetSelectable(false))
	table.SetCell(0, 2, tview.NewTableCell("Available").SetTextColor(tcell.ColorYellow).SetSelectable(false))

	refreshData := func() {

		table.SetTitle(fmt.Sprintf(" Deployments in namespace: [%s] ", a.Namespace()))

		for r := table.GetRowCount() - 1; r > 0; r-- {
			table.RemoveRow(r)
		}

		deploys, err := k8s.ListDeployments(a.Namespace())
		if err != nil {
			table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf("Error: %v", err)).SetTextColor(tcell.ColorRed))
			return
		}

		if len(deploys) == 0 {
			table.SetCell(1, 1, tview.NewTableCell("Deployments not found").SetTextColor(tcell.ColorOrange))
			return
		}

		for i, d := range deploys {
			row := i + 1
			table.SetCell(row, 0, tview.NewTableCell(d.Name).SetTextColor(tcell.ColorWhite))

			readyText := fmt.Sprintf("%d/%d", d.Available, d.Replicas)
			color := tcell.ColorGreen
			if d.Available < d.Replicas {
				color = tcell.ColorRed
			}
			table.SetCell(row, 1, tview.NewTableCell(readyText).SetTextColor(color))
			table.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf("%d", d.Available)).SetTextColor(tcell.ColorWhite))
		}
	}

	// Register refresher with ScreenDeployments
	a.register(ScreenDeployments, refreshData)
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
