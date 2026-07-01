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

// ScreenType defines the type of screen that is currently displayed.
type ScreenType int

const (
	ScreenPods ScreenType = iota
	ScreenDeployments
)

// String returns a human-readable name for the screen type.
func (s ScreenType) String() string {
	switch s {
	case ScreenPods:
		return "Pods"
	case ScreenDeployments:
		return "Deployments"
	default:
		return "Unknown"
	}
}

// RefreshRequest represents a request to refresh a specific screen.
type RefreshRequest struct {
	Screen    ScreenType
	Refresher func()
}

// App is the single source of truth for shared UI state (like the current
// namespace). Views read that state through it and subscribe to changes so
// every pane stays in sync.
type App struct {
	app         *tview.Application
	pages       *tview.Pages
	mainGrid    *tview.Grid
	headerLeft  *tview.TextView
	headerRight *tview.TextView

	podsTable        *tview.Table
	deploymentsTable *tview.Table

	namespace    string
	activeScreen ScreenType
	refreshers   []RefreshRequest
}

func New(cfg Config) *App {
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}
	return &App{
		app:          tview.NewApplication(),
		pages:        tview.NewPages(),
		mainGrid:     tview.NewGrid(),
		namespace:    cfg.Namespace,
		activeScreen: ScreenPods, // По умолчанию показываем поды
	}
}

func (a *App) Namespace() string        { return a.namespace }
func (a *App) ActiveScreen() ScreenType { return a.activeScreen }

// register subscribes a view's redraw function to namespace changes.
func (a *App) register(screen ScreenType, refresh func()) {
	a.refreshers = append(a.refreshers, RefreshRequest{Screen: screen, Refresher: refresh})
}

func (a *App) SetNamespace(ns string) {
	a.namespace = ns
	a.updateHeader()
	a.refreshActiveOnly()
}

func (a *App) refreshActiveOnly() {
	for _, req := range a.refreshers {
		if req.Screen == a.activeScreen {
			req.Refresher()
		}
	}
}

func (a *App) updateHeader() {

	headerArt := `    __   __ _    _  _____ ____  _   _ ___ 
	\ \ / // \  | |/ ( _ ) ___|| | | |_ _|
	 \ V // _ \ | ' // _ \___ \| | | || | 
	  | |/ ___ \| . \ (_) |__) | |_| || | 
	  |_/_/   \_\_|\_\___/____/ \___/|___|
	    Yust another k8s User Interface`
	a.headerLeft.SetText(headerArt).SetTextColor(tcell.ColorLightCyan)
	status := fmt.Sprintf("View: [green]%s[white] | Namespace: [yellow]%s[white]", a.ActiveScreen(), a.Namespace())
	a.headerRight.SetText(status)

}

func (a *App) Run() error {
	a.headerLeft = tview.NewTextView().SetDynamicColors(true)
	a.headerRight = tview.NewTextView().SetTextAlign(tview.AlignRight).SetDynamicColors(true)
	a.updateHeader()

	headerFlex := tview.NewFlex().
		AddItem(a.headerLeft, 0, 1, false).
		AddItem(a.headerRight, 0, 1, false)

	footerLeft := tview.NewTextView().SetDynamicColors(true)
	footerRight := tview.NewTextView().SetTextAlign(tview.AlignRight).SetDynamicColors(true)
	footerLeft.SetText("[[[yellow]n[white]]]amespace | [[[yellow]d[white]]]eployments | [[[yellow]p[white]]]ods")
	footerRight.SetText("[[[yellow]r[white]]]efresh | [[[yellow]Esc/Ctrl+C[white]]] Quit")

	footerFlex := tview.NewFlex().
		AddItem(footerLeft, 0, 1, false).
		AddItem(footerRight, 0, 1, false)

	a.podsTable = newPodsTable(a)
	a.deploymentsTable = newDeploymentsTable(a)

	a.mainGrid.
		SetRows(5, 0, 1).
		SetColumns(0).
		SetBorders(false)

	a.mainGrid.AddItem(headerFlex, 0, 0, 1, 1, 0, 0, false)
	a.mainGrid.AddItem(a.podsTable, 1, 0, 1, 1, 0, 0, true)
	a.mainGrid.AddItem(footerFlex, 2, 0, 1, 1, 0, 0, false)

	a.pages.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Rune() {
		case 'p', 'P':
			a.switchToPods()
			return nil
		case 'd', 'D':
			a.switchToDeployments()
			return nil
		case 'n', 'N':
			if name, _ := a.pages.GetFrontPage(); name == "main" {
				a.showNamespacePicker()
			}
			return nil
		case 'r', 'R':
			a.refreshActiveOnly()
			return nil
		}
		return ev
	})

	a.pages.AddPage("main", a.mainGrid, true, true)

	a.app.SetRoot(a.pages, true)
	a.app.SetFocus(a.podsTable)

	return a.app.Run()
}

func (a *App) switchToPods() {
	if a.activeScreen == ScreenPods {
		return
	}
	a.activeScreen = ScreenPods
	a.updateHeader()

	a.mainGrid.RemoveItem(a.deploymentsTable)
	a.mainGrid.AddItem(a.podsTable, 1, 0, 1, 1, 0, 0, true)

	a.app.SetFocus(a.podsTable)
	a.refreshActiveOnly()
}

func (a *App) switchToDeployments() {
	if a.activeScreen == ScreenDeployments {
		return
	}
	a.activeScreen = ScreenDeployments
	a.updateHeader()

	a.mainGrid.RemoveItem(a.podsTable)
	a.mainGrid.AddItem(a.deploymentsTable, 1, 0, 1, 1, 0, 0, true)

	a.app.SetFocus(a.deploymentsTable)
	a.refreshActiveOnly()
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
