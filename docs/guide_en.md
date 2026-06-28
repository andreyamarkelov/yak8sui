# Building yak8sui: A Beginner's Guide to a Kubernetes TUI in Go

> **Who is this for?** Developers who are new to Go. This document combines
> everything behind `yak8sui` — talking to Kubernetes, drawing a terminal UI,
> and the architecture that ties it together — and explains the *why* behind
> each decision, not just the *what*.

The project is a small terminal app (TUI) that lists Kubernetes pods and lets
you switch namespaces on the fly, similar to the popular tool `k9s`.

---

## Table of Contents

1. [The big picture: three layers](#1-the-big-picture-three-layers)
2. [The data layer: talking to Kubernetes (`client-go`)](#2-the-data-layer-talking-to-kubernetes-client-go)
3. [The UI layer: drawing with `tview`](#3-the-ui-layer-drawing-with-tview)
4. [The refactor: `App` as the single source of truth](#4-the-refactor-app-as-the-single-source-of-truth)
5. [Putting it together: how a namespace switch flows](#5-putting-it-together-how-a-namespace-switch-flows)
6. [Common pitfalls](#6-common-pitfalls)
7. [Resources](#7-resources)

---

## 1. The big picture: three layers

We deliberately split the program into three responsibilities. Each one only
knows about the layer directly below it:

```
cmd/yak8sui   (entry point)   →  start the app, pass in config
     │
pkg/ui        (presentation)  →  draw tables, handle keys  [knows tview]
     │
pkg/k8s       (data)          →  fetch pods/namespaces     [knows client-go]
     │
Kubernetes API
```

The golden rule:

- **`pkg/k8s` never imports `tview`** — it is pure data, testable without a terminal.
- **`pkg/ui` never imports `client-go`** — it only knows about simple Go structs.

Keeping these boundaries clean is what makes the project easy to extend later
(e.g. adding a "deployments" view).

### Why a directory per package?

In Go, **a package is a directory**, not a single file. That is why the pod
logic lives in `pkg/k8s/` and the UI lives in `pkg/ui/`. Each new responsibility
gets its own folder. Inside a folder you can have many files — they all share the
same package.

### Why `cmd/yak8sui/main.go` and not `cmd/main.go`?

The directory name becomes the binary name. `cmd/yak8sui/` produces a binary
called `yak8sui`. The `cmd/` folder is a *container* for one-or-more programs.
The one hard rule Go enforces: **one `main` package per directory.** Reserving
`cmd/yak8sui/` now means you can add a second binary later without restructuring.

---

## 2. The data layer: talking to Kubernetes (`client-go`)

All Kubernetes communication is isolated in `pkg/k8s`. The UI calls simple
functions like `ListPods(namespace)` and gets back plain Go structs.

### 2.1 Two libraries, two jobs

To talk to Kubernetes you need two separate external packages with clearly
divided responsibilities:

| Package | Role | Analogy |
|---|---|---|
| `k8s.io/apimachinery` | Foundation & rules. Defines what objects look like (e.g. `metav1.ListOptions`). Knows nothing about the network. | The rules for filling out an order form |
| `k8s.io/client-go` | Transport & network. Reads `~/.kube/config`, authenticates, sends HTTP requests. | The courier who delivers the form |

Neither works without the other.

### 2.2 The `client-go` call chain

The core of every request is one long method chain:

```go
clientset, err := kubernetes.NewForConfig(config)
pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
```

Reading it left to right:

- **`clientset`** — the main "control panel" holding mini-clients for every resource type.
- **`.CoreV1()`** — selects the `Core` API group, version `v1` (where pods, services, nodes live).
- **`.Pods(namespace)`** — narrows the scope to one namespace.
- **`.List(...)`** — the final action that actually sends the HTTP request to the API server.

### 2.3 `context.Background()`

The `context` package is Go's standard way to manage the lifecycle and
cancellation of network operations. If the API server hangs, a context lets you
abort by timeout instead of freezing forever.

`context.Background()` is a completely empty, default context — no timeout, can't
be cancelled. We pass it as a placeholder when we don't need advanced
cancellation. (A natural next step in your learning is to swap it for a context
with a timeout.)

### 2.4 Returning structured data, not just strings

A function shouldn't return a bare `[]string` of names when callers need more.
Instead we define a small package-level struct:

```go
type PodInfo struct {
    Name   string
    Status string
}

func ListPods(namespace string) ([]PodInfo, error) { ... }
```

A **struct** is Go's user-defined composite type: it groups logically related
variables (here, a pod's name and status) into one "box". Unlike OOP classes,
there's no hidden magic and no classical inheritance — methods are attached to
structs from the outside.

### 2.5 Bonus: struct embedding (why `pod.Name` works)

When you debug a `v1.Pod`, you only see `TypeMeta`, `ObjectMeta`, `Spec`, and
`Status` — yet the code writes `pod.Name`. That's **struct embedding**:

```go
type Pod struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`  // Name lives in here
    Spec              PodSpec
    Status            PodStatus
}
```

The first two fields have no name — they are **anonymous (embedded) fields**. Go
automatically **promotes** their inner fields to the top level, so both of these
are legal:

1. Full path: `pod.ObjectMeta.Name`
2. Shortcut: `pod.Name`

> A debugger shows the *physical* memory layout, so to find `Name` you must
> expand `ObjectMeta`. If two embedded structs had a field with the same name,
> Go would refuse `pod.Name` with an `ambiguous selector` error and you'd have to
> write the full path.

### 2.6 A shared client constructor

Both `ListPods` and `ListNamespaces` need a `clientset`, so the setup
(find `~/.kube/config` → build config → create clientset) lives in one private
helper, `newClientset()`, inside `client.go`. This avoids repeating the same code
in every data function.

---

## 3. The UI layer: drawing with `tview`

### 3.1 Why `tview` (and not Bubble Tea)?

For someone new to Go, the barrier to entry matters most:

- **Bubble Tea** uses the reactive Elm architecture (Model-View-Update). It's
  conceptually beautiful but forces you to manage streams of messages (`Msg`) and
  commands (`Cmd`) up front, which overloads beginners.
- **`tview`** offers a classic imperative, LEGO-like approach: you create ready-made
  widgets (tables, lists, grids), set their properties top-down, and mount them.
  The code stays linear and easy to debug.

### 3.2 The building blocks

| Widget | Used for |
|---|---|
| `tview.NewTable()` | The interactive list of pods, with row selection |
| `tview.NewTextView()` | Header art, the namespace/status panel, and the footer hints |
| `tview.NewGrid()` | Arranging header / table / footer into rows and columns |
| `tview.NewList()` | The namespace picker popup |
| `tview.NewPages()` | Stacking the popup *on top of* the main screen |

### 3.3 Refreshing data with a closure

Instead of drawing the table once, the row-filling logic is wrapped in a closure
called `refreshData()`. Each call:

1. Updates the table title to the current namespace.
2. Clears old rows (keeping the header) with `table.RemoveRow()`.
3. Fetches a fresh slice of pods from `pkg/k8s`.
4. Re-creates cells, applying **color coding by status** via `tcell` (Running →
   green, Pending → yellow, else red).

### 3.4 Handling keyboard input

`SetInputCapture` defines reactions to key events. On the pods table:

- `Esc` / `Ctrl+C` → `app.Stop()` exits cleanly.
- `r` / `R` → re-runs `refreshData()` for a manual refresh.

---

## 4. The refactor: `App` as the single source of truth

This is the most important architectural idea in the project, so we'll go slow.

### 4.1 The problem

Originally, each view received the namespace **as a string copied into its
closure**:

```go
func newPodsTable(app *tview.Application, namespace string) *tview.Table {
    // `namespace` is a private copy, frozen forever
}
```

That's fine for *one* static view. But the moment you want to **switch
namespaces and have every pane update**, copies become a trap:

- The pods table has its own copy.
- A future deployments table would have *another* copy.
- Nothing shares state, so changing one can't update the others.

### 4.2 The solution: centralized state + observers

We introduce an `App` struct that **owns** the shared state and a list of
"refresh me" callbacks. This is the classic **observer pattern** (the same idea
behind Redux/MVU on the frontend).

```go
// App is the single source of truth for shared UI state (like the current
// namespace). Views read that state through it and subscribe to changes so
// every pane stays in sync.
type App struct {
    app         *tview.Application
    pages       *tview.Pages
    headerRight *tview.TextView
    namespace   string      // ← the one and only copy
    refreshers  []func()    // ← every view's "redraw yourself" function
}
```

Three small methods make the whole pattern work:

```go
// Views READ the namespace through this — never their own copy.
func (a *App) Namespace() string { return a.namespace }

// A view SUBSCRIBES by handing over its redraw function.
func (a *App) register(refresh func()) {
    a.refreshers = append(a.refreshers, refresh)
}

// The ONE place the namespace changes. It updates state, then notifies everyone.
func (a *App) SetNamespace(ns string) {
    a.namespace = ns
    a.updateHeader()
    for _, refresh := range a.refreshers {
        refresh()
    }
}
```

### 4.3 How a view plugs in

A view now takes `*App` instead of a `namespace` string. It **reads** the
namespace live and **registers** itself so it gets redrawn on every change:

```go
func newPodsTable(a *App) *tview.Table {
    refreshData := func() {
        // always asks for the CURRENT namespace, never a stale copy
        pods, err := k8s.ListPods(a.Namespace())
        // ...fill rows...
    }

    a.register(refreshData) // subscribe to namespace changes
    refreshData()           // draw once immediately
    // ...
}
```

### 4.4 Why a global keybinding

The "switch namespace" key must work no matter which pane is focused. So it is
captured on the **application** itself, not on any single widget:

```go
a.app.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
    if ev.Rune() == 'n' || ev.Rune() == 'N' {
        if name, _ := a.pages.GetFrontPage(); name == "main" {
            a.showNamespacePicker()
            return nil
        }
    }
    return ev
})
```

The `GetFrontPage() == "main"` check stops the picker from re-opening while it's
already on screen.

### 4.5 The popup, via `Pages`

`tview.Pages` lets us stack screens. The namespace picker is a `tview.List`
fetched from the cluster, centered with a small `modal()` helper that uses empty
flex items as spacers. Selecting an item calls `a.SetNamespace(...)` and removes
the page.

### 4.6 Why this scales

| Concern | How it's handled |
|---|---|
| All panes use the same namespace | They call `a.Namespace()`, never a copy |
| Switching updates everything | `SetNamespace` loops over `refreshers` |
| Works from any focused pane | Key is captured on `app`, not a widget |
| Adding a deployments view later | It just calls `a.register(refresh)` — free updates |

Adding a new view that follows the global namespace is now trivial:

```go
func newDeploymentsTable(a *App) *tview.Table {
    refreshData := func() { /* k8s.ListDeployments(a.Namespace()) */ }
    a.register(refreshData) // 'n' now refreshes this view too, automatically
    // ...
}
```

---

## 5. Putting it together: how a namespace switch flows

Here is the full chain of events when the user presses `n` and picks a namespace:

```
User presses 'n'
   │
   ▼
app.SetInputCapture fires  →  a.showNamespacePicker()
   │
   ▼
k8s.ListNamespaces()  →  popup tview.List of namespaces
   │
   ▼
User selects "kube-system"  →  a.SetNamespace("kube-system")
   │
   ├─ a.namespace = "kube-system"      (update the single source of truth)
   ├─ a.updateHeader()                 (header now shows the new namespace)
   └─ for each refresher: refresh()    (every view redraws itself)
          │
          ▼
      pods table calls k8s.ListPods("kube-system")  →  new rows appear
```

One state change, every subscribed view stays in sync. That's the payoff.

---

## 6. Common pitfalls

### `terminal entry not found: term not set` in the IDE
Running a TUI from the GoLand/VSCode debug console can crash because that console
doesn't report a terminal type.
**Fix:** set the env var `TERM=xterm-256color` in the run configuration and
enable "Emulate terminal in output console".

### Disappearing `[r]` text in the footer
With `SetDynamicColors(true)`, the text `[r]` is parsed as a *color tag* (and
vanishes). Two options: disable dynamic colors for that view, or escape the
brackets by doubling them — `[[r]]` renders as a literal `[r]`. (That's exactly
why the footer in `app.go` writes `[[yellow]n[-]]`.)

### Capturing the loop variable in a closure
When building list items in a `for` loop, each closure must capture its *own*
copy of the loop variable:

```go
for _, ns := range namespaces {
    ns := ns // capture this iteration's value
    list.AddItem(ns, "", 0, func() { a.SetNamespace(ns) })
}
```

Without the `ns := ns` line (on older Go versions), every item would switch to
the *last* namespace.

---

## 7. Resources

- [pkg.go.dev](https://pkg.go.dev/) — official Go documentation search (including `context`, `os`, `fmt`).
- [Kubernetes API Reference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/) — field-by-field reference for Pod, Service, Deployment.
- [client-go examples](https://github.com/kubernetes/client-go/tree/master/examples) — official practical code samples.
- [tview](https://github.com/rivo/tview) and [tcell](https://github.com/gdamore/tcell) — the TUI and terminal libraries used here.
