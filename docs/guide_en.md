# Building yak8sui: A Beginner's Guide to a Kubernetes TUI in Go

> **Who is this for?** Developers who are new to Go. This document combines
> everything behind `yak8sui` — talking to Kubernetes, drawing a terminal UI,
> and the architecture that ties it together — and explains the *why* behind
> each decision, not just the *what*.

The project is a small terminal app (TUI) that lists Kubernetes **pods and
deployments**, lets you **switch between those views** with a keypress, and lets
you **switch namespaces** on the fly — similar to the popular tool `k9s`.

---

## Table of Contents

1. [The big picture: three layers](#1-the-big-picture-three-layers)
2. [The data layer: talking to Kubernetes (`client-go`)](#2-the-data-layer-talking-to-kubernetes-client-go)
3. [The UI layer: drawing with `tview`](#3-the-ui-layer-drawing-with-tview)
4. [The refactor: `App` as the single source of truth](#4-the-refactor-app-as-the-single-source-of-truth)
5. [Go mechanics, up close](#5-go-mechanics-up-close)
6. [Putting it together: how a namespace switch flows](#6-putting-it-together-how-a-namespace-switch-flows)
7. [Common pitfalls](#7-common-pitfalls)
8. [Resources](#8-resources)

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

### The project file map

Here is every file and what it's responsible for. Keep this handy — the rest of
the guide refers to these files by name:

```
yak8sui/
├── cmd/
│   └── yak8sui/
│       └── main.go         # entry point: builds App and calls Run()
├── pkg/
│   ├── k8s/                # DATA layer (package k8s) — knows client-go
│   │   ├── client.go       #   newClientset(): kubeconfig → clientset
│   │   ├── pods.go         #   PodInfo struct + ListPods(namespace)
│   │   ├── deployments.go  #   DeploymentInfo struct + ListDeployments(namespace)
│   │   └── namespaces.go   #   ListNamespaces()
│   └── ui/                 # UI layer (package ui) — knows tview
│       ├── app.go          #   App struct, New(), Run(), SetNamespace(), screen
│       │                   #   switching, the global keybindings, picker, modal()
│       ├── pods.go         #   newPodsTable(): the pods table + refreshData()
│       ├── deployments.go  #   newDeploymentsTable(): the deployments table
│       └── colors.go       #   statusColor(): maps a pod status to a color
├── go.mod                  # module name + dependency versions
└── go.sum                  # cryptographic checksums of those dependencies
```

A quick way to read it: **one folder per package**, **one file per responsibility**.
This is exactly how the deployments view was added — a `pkg/k8s/deployments.go`
(data) and a `pkg/ui/deployments.go` (view), with **no changes** needed to the
existing pods files.

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

### 2.6 A shared client constructor — and the Go error pattern

Both `ListPods` and `ListNamespaces` need a `clientset`, so the setup lives in one
private helper, `newClientset()`, inside `client.go`:

```go
func newClientset() (*kubernetes.Clientset, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, fmt.Errorf("failed to get home dir: %w", err)
    }

    kubeconfig := filepath.Join(home, ".kube", "config")

    config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
    if err != nil {
        return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
    }

    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
    }

    return clientset, nil
}
```

Centralizing this means `ListPods` and `ListNamespaces` don't each repeat the
setup. It also shows the **Go error-handling idiom** you'll see everywhere:

- Go has no exceptions. A function that can fail returns an `error` as its **last
  return value**; `nil` means "success, no error."
- After each fallible call you check `if err != nil` and **return early**. That's
  why Go reads as a flat "do a step, check the error" sequence instead of nested
  try/catch blocks.
- `fmt.Errorf("...: %w", err)` **wraps** the original error with a human-readable
  message. The `%w` verb keeps the underlying error attached, so a caller can both
  read your message *and* inspect the original cause.
- On success the final line returns the real value plus `nil`.

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
4. Re-creates cells, applying **color coding by status** via the `statusColor`
   helper in `colors.go` (Running/Succeeded → green, Pending → yellow, else red).

### 3.4 Handling keyboard input

`SetInputCapture` defines reactions to key events. On the pods table:

- `Esc` / `Ctrl+C` → `app.Stop()` exits cleanly.
- `r` / `R` → re-runs `refreshData()` for a manual refresh.

---

## 4. The refactor: `App` as the single source of truth

This is the most important architectural idea in the project, so we'll go slow.

### 4.1 The problem

Before this refactor, each view received the namespace **as a string copied into
its closure** (this is the *old* signature — you won't find it in the code now,
we're explaining what we moved away from):

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

We introduce an `App` struct that **owns** the shared state (the current
namespace *and* which screen is active) and a list of "refresh me" callbacks.
This is the classic **observer pattern** (the same idea behind Redux/MVU on the
frontend).

First, two small helper types. Since the app now has more than one screen, we
label each screen with a `ScreenType` and tag every refresher with the screen it
belongs to:

```go
type ScreenType int

const (
    ScreenPods ScreenType = iota
    ScreenDeployments
)

// Each registered refresher remembers which screen it belongs to.
type RefreshRequest struct {
    Screen    ScreenType
    Refresher func()
}
```

Now the `App` struct itself:

```go
// App is the single source of truth for shared UI state (the current namespace
// and which screen is active). Views read that state through it and subscribe to
// changes so the visible pane stays in sync.
type App struct {
    app         *tview.Application
    pages       *tview.Pages
    mainGrid    *tview.Grid
    headerLeft  *tview.TextView
    headerRight *tview.TextView

    podsTable        *tview.Table
    deploymentsTable *tview.Table

    namespace    string           // ← the one and only copy
    activeScreen ScreenType       // ← Pods or Deployments
    refreshers   []RefreshRequest // ← each view's redraw fn, tagged by screen
}
```

The methods that make the pattern work:

```go
// Views READ the namespace through this — never their own copy.
func (a *App) Namespace() string { return a.namespace }

// A view SUBSCRIBES by handing over its redraw function, tagged with its screen.
func (a *App) register(screen ScreenType, refresh func()) {
    a.refreshers = append(a.refreshers, RefreshRequest{Screen: screen, Refresher: refresh})
}

// Changing the namespace refreshes only the screen that's currently visible.
func (a *App) SetNamespace(ns string) {
    a.namespace = ns
    a.updateHeader()
    a.refreshActiveOnly()
}

// refreshActiveOnly runs the redraw functions for the active screen only.
func (a *App) refreshActiveOnly() {
    for _, req := range a.refreshers {
        if req.Screen == a.activeScreen {
            req.Refresher()
        }
    }
}
```

> **Why refresh only the active screen?** Only one table is visible at a time
> (pods *or* deployments). Redrawing the hidden one would waste work *and* fire an
> extra network call to Kubernetes for data nobody is looking at. So each
> refresher is tagged with its screen, and `refreshActiveOnly` skips the rest. When
> you switch screens, that switch calls `refreshActiveOnly` too, so the
> newly-shown table loads fresh data on demand.

Picture it as a hub-and-spoke. At startup every view **subscribes** (tagged by
screen); later one `SetNamespace` call **broadcasts — but only to the active
screen's refreshers**:

```
   REGISTRATION  (each view calls a.register(SCREEN, refreshData) at startup)

   ┌────────────────┐        ┌─────────────────────┐
   │ pods table     │        │ deployments table   │
   │ {Pods, refresh}│        │ {Deployments,refresh}│
   └───────┬────────┘        └──────────┬──────────┘
           │                            │
           └─────────────┬──────────────┘
                         ▼
             ┌────────────────────────┐
             │          App           │
             │  namespace             │
             │  activeScreen = Pods   │
             │  refreshers            │
             │    []RefreshRequest    │
             └───────────┬────────────┘

   BROADCAST  (SetNamespace runs only refreshers whose Screen == activeScreen)

                         │  SetNamespace("kube-system")
                         │  → refreshActiveOnly()
                         ▼
          ╔═════════════════════════════════════════╗
          ║ only the ACTIVE screen's refresh() runs ║
          ║ (the hidden table is skipped)           ║
          ╚═════════════════════════════════════════╝
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

    a.register(ScreenPods, refreshData) // subscribe (tagged as the Pods screen)
    refreshData()                       // draw once immediately
    // ...
}
```

### 4.4 Global keybindings

The keys must work no matter which pane is focused, so they're captured once — on
the `Pages` container that holds everything, not on any single widget:

```go
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
```

- `p` / `d` — switch between the Pods and Deployments views.
- `n` — open the namespace picker (only when the `"main"` page is in front, so it
  can't re-open on top of itself).
- `r` — manually refresh the active screen.

### 4.5 Switching screens

Because both tables are built once and stored on the `App` (`podsTable`,
`deploymentsTable`), switching a view is just swapping which table sits in the
middle row of the grid:

```go
func (a *App) switchToDeployments() {
    if a.activeScreen == ScreenDeployments {
        return // already here, nothing to do
    }
    a.activeScreen = ScreenDeployments
    a.updateHeader()

    a.mainGrid.RemoveItem(a.podsTable)
    a.mainGrid.AddItem(a.deploymentsTable, 1, 0, 1, 1, 0, 0, true)

    a.app.SetFocus(a.deploymentsTable)
    a.refreshActiveOnly() // load fresh data for the newly-shown screen
}
```

`switchToPods` is the mirror image. Each starts with an "am I already here?"
guard so pressing the same key twice does nothing.

### 4.6 The popup, via `Pages`

`tview.Pages` lets us stack screens. The namespace picker is a `tview.List`
fetched from the cluster, centered with a small `modal()` helper that uses empty
flex items as spacers. Selecting an item calls `a.SetNamespace(...)` and removes
the page.

### 4.7 Why this scales

| Concern | How it's handled |
|---|---|
| All panes use the same namespace | They call `a.Namespace()`, never a copy |
| Switching namespace redraws the view | `SetNamespace` → `refreshActiveOnly()` |
| Only the visible screen does work | Refreshers are tagged by `ScreenType` |
| Switching between views | `p` / `d` swap which table sits in the grid |
| Works from any focused pane | Keys are captured on `pages`, not a widget |
| Adding another view later | Build a table + `a.register(screen, refresh)` |

Adding the deployments view *did* follow this recipe — that's not hypothetical
anymore:

```go
func newDeploymentsTable(a *App) *tview.Table {
    refreshData := func() { /* k8s.ListDeployments(a.Namespace()) ... */ }
    a.register(ScreenDeployments, refreshData) // tagged as the Deployments screen
    refreshData()
    // ...
}
```

---

## 5. Go mechanics, up close

This section zooms in on the Go language features that make the architecture
above possible. If you're new to Go, these are the "aha" concepts.

> **New to pointers?** A few symbols show up all over this code:
> - `*App` means "a pointer to an `App`" — an arrow pointing at the real struct in
>   memory, not a copy of it.
> - `&App{...}` means "create an `App` and give me its address" (a pointer to it).
> - `a.namespace` reads a field *through* the pointer — Go follows the arrow for you,
>   so you don't need special syntax.
>
> Why bother instead of just copying the struct? Two reasons: (1) copying a big
> struct on every call is wasteful, and (2) more importantly, if each view had its
> own *copy* of `App`, changing `namespace` in one wouldn't affect the others. The
> entire design depends on **one shared `App`** — pointers guarantee everyone is
> looking at the same box in memory.

### 5.1 Methods and receivers: how behavior attaches to data

In OOP languages (Java, C#, Python) methods are written *inside* the class body.
Go separates data and behavior physically, then links them with a **receiver**.

A struct holds only data:

```go
type App struct {
    app   *tview.Application
    pages *tview.Pages
    // ...
}
```

A method is a normal function with an extra `(a *App)` before its name:

```go
func (a *App) Run() error {
    // ...
}
```

That `(a *App)` is the glue. It's a **pointer receiver**, and it tells Go: "this
`Run` function belongs to `App`, and inside it `a` refers to the specific instance
it was called on." So when `main.go` does:

```go
app := ui.New(ui.Config{Namespace: "kube-system"})
err := app.Run()
```

Go passes your `app` variable into `Run` as `a`. Inside `Run` you can then write
`a.app.SetRoot(...)` or `a.showNamespacePicker()`.

> **Why a pointer (`*App`) and not a value (`App`)?** A pointer receiver lets a
> method *modify* the struct — `SetNamespace` needs to change `a.namespace`, which
> only works through a pointer. It also avoids copying the whole struct on every
> call. Rule of thumb: if a struct has more than a handful of fields or any method
> mutates it, use a pointer receiver.
>
> **Why `App` is capitalized but `namespace` is not:** In Go, an identifier that
> starts with an **uppercase letter is exported** (public — usable from other
> packages); **lowercase is unexported** (private — visible only inside its own
> package). So `App`, `Run`, and `Namespace` are public, while `namespace` and
> `register` are private to `pkg/ui`. That's exactly why views must read the
> namespace through the `Namespace()` method instead of touching `a.namespace`
> directly.

A practical benefit: **methods of one struct can live in different files** of the
same package. Every function that starts with `(a *App)` attaches to the same
`App` struct, so you can spread them across files instead of bloating one. That's
why `Run`, `SetNamespace`, and `showNamespacePicker` can all be methods on `App`.

### 5.2 The constructor pattern: `New` + `Config`

Go has no built-in constructors, so by convention we write a `New` function. Watch
the string `"kube-system"` travel from `main.go` into the running app:

```
1. In main.go:            "kube-system"
      ▼
2. Wrapped in a struct:   ui.Config{Namespace: "kube-system"}
      ▼
3. Passed as an argument:  ui.New( ui.Config{...} )
      ▼
4. Arrives inside New() as the parameter: cfg
      ▼
5. Copied into the struct's private field: a.namespace
      ▼
6. Run() can now always read the current namespace
```

The receiving end:

```go
func New(cfg Config) *App {
    return &App{
        app:       tview.NewApplication(),
        namespace: cfg.Namespace, // ← the link
    }
}
```

Why wrap arguments in a `Config` struct instead of passing them directly? It makes
adding future options (context name, refresh interval…) painless without changing
`New`'s signature. Note the link is **strict**: a typo like `ui.Config{NameSpace: ...}`
(capital S) or a non-existent field won't compile — Go checks field names exactly.

> **`:=` vs `=`:** The `:=` operator (used in `main.go` as `app := ui.New(...)`)
> declares a new variable *and* infers its type from the value on the right. Use
> plain `=` only to reassign a variable that was *already declared*. If you wrote
> `var app *ui.App` on its own line, the next line would be `app = ui.New(...)`.
> Also note `&App{...}` — the `&` takes the address of the struct literal, so
> `New` returns a pointer (`*App`), not a copy.

### 5.3 What's in memory, and the `Run` event loop

After startup, exactly one big box lives in memory — the `app` variable of type
`*ui.App`:

```
[ app (*ui.App) in memory ]
├── namespace        = "kube-system"
├── activeScreen     = ScreenPods
├── app              = [the running tview.Application]
├── pages            = [page manager, currently holding the main Grid]
├── mainGrid         = [header / active table / footer]
├── headerLeft       = [the ASCII-art title]
├── headerRight      = [status: active screen + namespace]
├── podsTable        = [the pods table widget]
├── deploymentsTable = [the deployments table widget]
└── refreshers       = [{Pods, refreshData}, {Deployments, refreshData}]
```

When `main.go` reaches `app.Run()`, control passes *into* `tview` and **stops
there**. The program enters an event loop — think of a guard in an infinite
`for {}` waiting for signals:

1. **Sleeps and waits** — almost no CPU used, just waiting for terminal events.
2. **Catches a keypress** — you press `n`.
3. **Wakes the input capture** — `tview` calls the function we attached in `app.go`.
4. **Changes the picture** — the handler tells `a.pages` to show the namespace list on top.
5. **Sleeps again** — until your next key or click.

This is a classic **event-driven application**: static *state* (the `App` struct)
plus an *engine* (`Run`) that only wakes on input, mutates state, and sleeps again.
`main.go` simply waits at `app.Run()` until the loop ends (e.g. via `a.app.Stop()`).

### 5.4 Functions as values (and closures)

In Go a function is a value, like a string or a number: you can store it in a
variable, put it in a slice, or pass it as an argument. That's what powers the
observer registry.

Think of `App` as a **magazine publisher** with a blank subscriber list
(`refreshers []RefreshRequest`). The pods table is a **reader**. When it calls
`a.register(ScreenPods, refreshData)`, it isn't asking for data now — it's handing
over its "business card" (the function, plus which screen it's for) to be called
later:

```go
type RefreshRequest struct {
    Screen    ScreenType
    Refresher func() // ← a FUNCTION stored as a value
}

func (a *App) register(screen ScreenType, refresh func()) {
    a.refreshers = append(a.refreshers, RefreshRequest{Screen: screen, Refresher: refresh})
}
```

The type `func()` means "a function taking no arguments and returning nothing."
Crucially, we pass `refreshData` **without** parentheses — `register(..., refreshData)`
hands over the function itself; `refreshData()` would *call* it and pass the result
instead. Later, `refreshActiveOnly` invokes the matching ones **with** parentheses:

```go
for _, req := range a.refreshers {
    if req.Screen == a.activeScreen {
        req.Refresher() // the () means "run it now"
    }
}
```

The magic is that `refreshData` is a **closure**: it "captured" the `table`
variable from `newPodsTable`. Even after we hand it to `App`, it still remembers
which table to clear and fill. And `App` has no idea `pods.go` even exists — it
just calls abstract functions. Register more tables (deployments, services…) and
the same loop refreshes whichever screen is active.

```
   newPodsTable(a *App)                          App
   ┌──────────────────────────────┐             ┌──────────────────┐
   │ table := tview.NewTable() ◀┐ │             │                  │
   │                            │ │             │ refreshers       │
   │ refreshData := func() {    │ │             │   []func()       │
   │     table.SetTitle(...) ───┘ │  capture    │                  │
   │     pods := k8s.ListPods(...)│             │                  │
   │     ...fill rows...          │             │                  │
   │ }                            │             │                  │
   │                              │             │                  │
   └──────────┬───────────────────┘             │                  │
              │  a.register(ScreenPods, refreshData)             │
              └─────────────────────────────────▶  filed here     │
                                               └──────────────────┘

   Later, App calls refreshData() from SetNamespace. The closure STILL
   remembers `table`, so it knows which table to clear and refill —
   even though App itself never saw `pods.go`.
```

### 5.5 Walkthrough: the namespace picker and the centering trick

`showNamespacePicker` (in `app.go`) builds the popup:

```go
list := tview.NewList().ShowSecondaryText(false)         // compact list
list.SetBorder(true).SetTitle(" Select namespace (Esc to cancel) ")

namespaces, err := k8s.ListNamespaces()                  // ask the cluster
```

On error it adds a single item showing the message; otherwise it adds one item
per namespace, each with a callback that fires on Enter:

```go
for _, ns := range namespaces {
    ns := ns // capture this iteration's value (see pitfalls)
    list.AddItem(ns, "", 0, func() {
        a.SetNamespace(ns)                // triggers the whole refresh chain
        a.pages.RemovePage("namespace")   // close the popup
    })
}
```

`Esc` removes the page without changing anything. Finally it mounts the list and
moves keyboard focus to it so the arrow keys drive the list, not the table behind it:

```go
a.pages.AddPage("namespace", modal(list, 40, 20), true, true)
a.app.SetFocus(list)
```

**The centering trick (`modal`):** terminals have no built-in "center this window."
`modal` nests two flex containers and uses `nil` items as invisible spacers — one
on each side horizontally, one above and below vertically — pinning a fixed-size
box (40×20) in the middle no matter how the terminal is resized.

---

## 6. Putting it together: how a namespace switch flows

Here is the full chain of events when the user presses `n` and picks a namespace:

```
User presses 'n'
   │
   ▼
pages.SetInputCapture fires  →  a.showNamespacePicker()
   │
   ▼
k8s.ListNamespaces()  →  popup tview.List of namespaces
   │
   ▼
User selects "kube-system"  →  a.SetNamespace("kube-system")
   │
   ├─ a.namespace = "kube-system"      (update the single source of truth)
   ├─ a.updateHeader()                 (header shows active screen + namespace)
   └─ a.refreshActiveOnly()            (only the VISIBLE screen redraws)
          │
          ▼
      active table calls k8s.List…(a.Namespace())  →  new rows appear
```

One state change, and the visible screen stays in sync. Switching screens with
`p` / `d` works the same way: it flips `activeScreen`, swaps the table in the grid,
and calls `refreshActiveOnly()` so the newly-shown table loads on demand.

---

## 7. Common pitfalls

### `terminal entry not found: term not set` in the IDE
Running a TUI from the GoLand/VSCode debug console can crash because that console
doesn't report a terminal type.
**Fix:** set the env var `TERM=xterm-256color` in the run configuration and
enable "Emulate terminal in output console".

### Disappearing `[r]` text in the footer
With `SetDynamicColors(true)`, text like `[r]` is parsed as a *color tag* (and
vanishes). The fix is to escape a literal bracket by doubling it — `[[` renders as
a single `[`. That's why the footer in `app.go` writes strings like
`[[[yellow]n[white]]]amespace`: the leading `[[` prints a real `[`, then
`[yellow]`/`[white]` switch colors to highlight the shortcut key.

### Capturing the loop variable in a closure
When you create a closure inside a `for` loop and the closure uses the loop
variable, you must make sure each closure captures *this iteration's* value, not
a single shared variable:

```go
for _, ns := range namespaces {
    ns := ns // make a per-iteration copy
    list.AddItem(ns, "", 0, func() { a.SetNamespace(ns) })
}
```

**The history:** In Go versions before 1.22, `ns` was a *single* variable reused
every iteration. All the closures would point at that one variable, so by the
time they ran (on Enter) the loop had finished and `ns` held the *last* value —
every item would switch to the last namespace. The `ns := ns` line shadowed it
with a fresh copy per iteration and fixed this.

**Since Go 1.22** (this project uses 1.26), loop variables are per-iteration by
default, so the `ns := ns` line is technically no longer needed here. We keep it
because it's harmless, makes the intent obvious to readers who learned on older
Go, and is a common idiom you'll meet in real codebases.

---

## 8. Resources

- [pkg.go.dev](https://pkg.go.dev/) — official Go documentation search (including `context`, `os`, `fmt`).
- [Kubernetes API Reference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.30/) — field-by-field reference for Pod, Service, Deployment.
- [client-go examples](https://github.com/kubernetes/client-go/tree/master/examples) — official practical code samples.
- [tview](https://github.com/rivo/tview) and [tcell](https://github.com/gdamore/tcell) — the TUI and terminal libraries used here.
