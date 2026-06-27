package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"yak8sui/pkg/k8s"
)

type Config struct {
	Namespace string
}

// App is the single source of truth for shared UI state (like the current
// namespace). Views read that state through it and subscribe to changes so
// every pane stays in sync.
type App struct {
	app         *tview.Application
	pages       *tview.Pages
	headerRight *tview.TextView
	namespace   string
	refreshers  []func()
}

func New(cfg Config) *App {
	return &App{
		app:       tview.NewApplication(),
		namespace: cfg.Namespace,
	}
}

func (a *App) Namespace() string { return a.namespace }

// register subscribes a view's redraw function to namespace changes.
func (a *App) register(refresh func()) {
	a.refreshers = append(a.refreshers, refresh)
}

// SetNamespace is the only place the namespace is mutated. It updates shared
// state and then asks every registered view to redraw itself.
func (a *App) SetNamespace(ns string) {
	a.namespace = ns
	a.updateHeader()
	for _, refresh := range a.refreshers {
		refresh()
	}
}

func (a *App) updateHeader() {
	if a.headerRight == nil {
		return
	}
	a.headerRight.SetText(fmt.Sprintf("\n\n CONTEXT:   ---\n NAMESPACE: %s\n STATUS:    [green]Connected?[-]", a.namespace))
}

func (a *App) Run() error {
	table := newPodsTable(a)

	headerArt := `    __   __ _    _  _____ ____  _   _ ___ 
	\ \ / // \  | |/ ( _ ) ___|| | | |_ _|
	 \ V // _ \ | ' // _ \___ \| | | || | 
	  | |/ ___ \| . \ (_) |__) | |_| || | 
	  |_/_/   \_\_|\_\___/____/ \___/|___|
	    Yust another k8s User Interface`

	headerLeft := tview.NewTextView().
		SetText(headerArt).
		SetTextColor(tcell.ColorLightGreen).
		SetWrap(false).
		SetWordWrap(false).
		SetTextAlign(tview.AlignLeft)

	a.headerRight = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	a.updateHeader()

	footer := tview.NewTextView().
		SetText(" [[yellow]n[-]] namespace | [[yellow]r[-]] refresh | [[red]Esc[-] / [red]Ctrl+C[-]] exit").
		SetDynamicColors(true)

	grid := tview.NewGrid().
		SetRows(7, 0, 1).
		SetColumns(60, 0).
		AddItem(headerLeft, 0, 0, 1, 1, 0, 0, false).
		AddItem(a.headerRight, 0, 1, 1, 1, 0, 0, false).
		AddItem(table, 1, 0, 1, 2, 0, 0, true).
		AddItem(footer, 2, 0, 1, 2, 0, 0, false)

	a.pages = tview.NewPages().AddPage("main", grid, true, true)

	// Global keybinding: 'n' opens the namespace picker regardless of which
	// pane is focused. We only trigger it from the main page so it can't
	// re-open while the picker itself is up.
	a.app.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Rune() == 'n' || ev.Rune() == 'N' {
			if name, _ := a.pages.GetFrontPage(); name == "main" {
				a.showNamespacePicker()
				return nil
			}
		}
		return ev
	})

	return a.app.SetRoot(a.pages, true).EnableMouse(true).Run()
}

func (a *App) showNamespacePicker() {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle(" Select namespace (Esc to cancel) ")

	namespaces, err := k8s.ListNamespaces()
	if err != nil {
		list.AddItem(fmt.Sprintf("Error: %v", err), "", 0, func() {
			a.pages.RemovePage("namespace")
		})
	} else {
		for _, ns := range namespaces {
			ns := ns // capture loop variable for the closure
			list.AddItem(ns, "", 0, func() {
				a.SetNamespace(ns)
				a.pages.RemovePage("namespace")
			})
		}
	}

	list.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEscape {
			a.pages.RemovePage("namespace")
			return nil
		}
		return ev
	})

	a.pages.AddPage("namespace", modal(list, 40, 20), true, true)
	a.app.SetFocus(list)
}

// modal centers a primitive on screen using nested flex layouts as spacers.
func modal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}
