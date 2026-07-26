# Table-Driven Tests in Go: Testing yak8sui

> **Who is this for?** Developers new to Go who want to understand how testing
> works in the language — no third-party framework, no assertions library, just
> the standard toolchain. We use two tiny functions from `yak8sui` as the
> subject and explain the *why* behind every line, not just the *what*.

If you have read the [main guide](guide_en.md), you know the project is split
into a **data** layer (`pkg/k8s`) and a **UI** layer (`pkg/ui`). That guide
mentioned in passing that pure data is "testable without a terminal." This
document makes good on that promise: it adds the project's **first two tests**
and teaches the idiom every Go codebase leans on — the **table-driven test**.

---

## Table of Contents

1. [What "testing" means in Go](#1-what-testing-means-in-go)
2. [The rules: file names, package names, function names](#2-the-rules-file-names-package-names-function-names)
3. [What we're testing, and why these two functions](#3-what-were-testing-and-why-these-two-functions)
4. [Anatomy of a table-driven test](#4-anatomy-of-a-table-driven-test)
5. [Our tests, line by line](#5-our-tests-line-by-line)
6. [Running the tests](#6-running-the-tests)
7. [Reading a failure](#7-reading-a-failure)
8. [Why table-driven? The payoff](#8-why-table-driven-the-payoff)
9. [Choosing good cases (the `default` branch trap)](#9-choosing-good-cases-the-default-branch-trap)
10. [The honest limitation: change-detector tests](#10-the-honest-limitation-change-detector-tests)
11. [How this plugs into CI](#11-how-this-plugs-into-ci)
12. [Where to go next](#12-where-to-go-next)
13. [Resources](#13-resources)

---

## 1. What "testing" means in Go

Coming from other languages, you might expect to `pip install` or `npm add` a
test framework (JUnit, pytest, Jest, Mocha…). In Go there is **nothing to
install**. Testing is built into the language and the toolchain:

- A package called **`testing`** ships with the standard library.
- The **`go test`** command discovers and runs your tests.
- There are **no assertion helpers** like `assertEquals`. You compare values
  with plain `if` statements and report failures with `t.Errorf`. This feels
  bare at first, but it means test code is *just Go code* — no new DSL to learn.

That philosophy — "less magic, more plain code" — is the same one you met in the
main guide with error handling (`if err != nil`) instead of exceptions.

---

## 2. The rules: file names, package names, function names

Go's tooling finds tests by **convention**, not configuration. Three rules:

### Rule 1 — the file must end in `_test.go`

Our files are `pkg/ui/colors_test.go` and `pkg/ui/app_test.go`. The `_test.go`
suffix is special: these files are compiled **only** when you run `go test`, and
are **excluded** from the normal `go build`. So test code never bloats your
shipped binary.

### Rule 2 — a test function is `func TestXxx(t *testing.T)`

- The name must start with `Test` followed by a capital letter (`TestStatusColor`,
  not `Teststatuscolor`).
- It takes exactly one argument: `t *testing.T`. That `t` is your handle to the
  test runner — you call `t.Run(...)` to make subtests and `t.Errorf(...)` to
  record a failure.

### Rule 3 — pick the package: white-box vs black-box

A `_test.go` file can declare one of two packages:

| Declaration | Name | Sees unexported (lowercase) names? | Called |
|---|---|---|---|
| `package ui` | internal test | **Yes** | white-box |
| `package ui_test` | external test | No — only the public API | black-box |

We chose **`package ui`** (white-box) for both files. Why? Because
`statusColor` is **unexported** (lowercase `s`) — from outside the package you
literally cannot name it. Since we want to test it directly, our test must live
*inside* `package ui`. `ScreenType.String()` is exported and could have gone
either way, but keeping both files in `package ui` is simpler and consistent.

> **Learning aside:** the main guide's section 5.1 explained that
> capitalization controls visibility (`App` public, `namespace` private). That
> same rule is exactly what forces this white-box choice here — a great example
> of a language rule having a concrete downstream consequence.

---

## 3. What we're testing, and why these two functions

We deliberately picked the two **purest** functions in the codebase:

**`statusColor`** — in `pkg/ui/colors.go`:

```go
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
```

**`ScreenType.String()`** — in `pkg/ui/app.go`:

```go
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
```

What makes them ideal first tests is that they are **pure functions**:

- **Same input → same output, every time.** No randomness, no clock, no global
  state.
- **No side effects.** They don't touch the network, the filesystem, or a
  terminal. Contrast this with `newPodsTable` or `k8s.ListPods`, which need a
  running Kubernetes cluster and a `tview` screen — testing *those* requires
  extra machinery (interfaces and fakes) that we deliberately skip for now.
- **No setup.** You call the function and inspect the return value. That's it.

A pure function is the "hello world" of testing: nothing to mock, nothing to
tear down. Master the pattern here and you can apply it anywhere.

> **A key property: these tests are 100% additive.** We wrote two brand-new
> `_test.go` files and changed **zero** lines of existing code. `colors.go` and
> `app.go` are byte-for-byte identical to before. The safest possible change —
> fully reversible by deleting the two test files.

---

## 4. Anatomy of a table-driven test

A **table-driven test** describes each scenario as a **row of data** in a slice,
then loops over the rows running the same assertion on each. The shape is so
common in Go that you'll recognize it instantly in any real codebase:

```go
func TestSomething(t *testing.T) {
    // 1. THE TABLE: a slice of anonymous structs, one row per scenario.
    cases := []struct {
        name string // what this row checks
        in   InputType
        want OutputType
    }{
        {name: "first scenario", in: ..., want: ...},
        {name: "second scenario", in: ..., want: ...},
    }

    // 2. THE LOOP: run every row.
    for _, tc := range cases {
        // 3. THE SUBTEST: t.Run isolates each row under its own name.
        t.Run(tc.name, func(t *testing.T) {
            got := Something(tc.in)
            if got != tc.want {
                t.Errorf("Something(%v) = %v, want %v", tc.in, got, tc.want)
            }
        })
    }
}
```

Three pieces to internalize:

1. **The table** is `[]struct{...}{...}` — an *anonymous* struct type defined and
   filled inline. The main guide (section 2.4) introduced named structs like
   `PodInfo`; here the struct has no name because it exists only to hold test
   rows. Each row conventionally has a `name`, some inputs, and the expected
   `want`.
2. **The loop** walks the rows with `for _, tc := range cases` (`tc` = "test
   case").
3. **`t.Run(name, func)`** creates a **subtest** — a named, independently
   reported child test. If row 4 fails, Go tells you exactly which one by name,
   and the other rows still run.

> **Note on the loop variable:** the main guide's pitfalls section warned about
> capturing a `for` loop variable in a closure. The same variable `tc` is
> captured by the `t.Run` closure here. On Go 1.22+ (this project is on 1.26)
> each iteration gets a fresh `tc`, so this is safe with no `tc := tc` line
> needed. On older Go you'd add `tc := tc` inside the loop.

---

## 5. Our tests, line by line

### 5.1 `colors_test.go`

```go
package ui

import (
    "testing"

    "github.com/gdamore/tcell/v2"
)

func TestStatusColor(t *testing.T) {
    cases := []struct {
        name   string      // human-readable description, shown in test output
        status string      // input to statusColor
        want   tcell.Color // the color we expect back
    }{
        {name: "running is green", status: "Running", want: tcell.ColorGreen},
        {name: "succeeded is green", status: "Succeeded", want: tcell.ColorGreen},
        {name: "pending is yellow", status: "Pending", want: tcell.ColorYellow},
        {name: "failed falls through to red", status: "Failed", want: tcell.ColorRed},
        {name: "unknown status is red", status: "CrashLoopBackOff", want: tcell.ColorRed},
        {name: "empty string is red", status: "", want: tcell.ColorRed},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := statusColor(tc.status)
            if got != tc.want {
                t.Errorf("statusColor(%q) = %v, want %v", tc.status, got, tc.want)
            }
        })
    }
}
```

Reading it:

- **`package ui`** — white-box, so we can call the unexported `statusColor`.
- **We import `tcell`** because the *expected* values are `tcell.Color`
  constants (`tcell.ColorGreen`, …). The test speaks the same vocabulary as the
  function under test.
- **The table has six rows** covering all three branches: both `case` values that
  map to green, the yellow case, and — crucially — **three** inputs that exercise
  the `default` branch (`"Failed"`, a made-up status, and `""`).
- **`got := statusColor(tc.status)`** runs the real function.
- **`if got != tc.want { t.Errorf(...) }`** is the whole "assertion." No library
  — just a comparison. `t.Errorf` records a failure *and lets the test keep
  going* (unlike `t.Fatalf`, which stops the current subtest immediately).
- **The format verbs** matter for readable failures: `%q` quotes the string (so
  an empty input prints as `""`, not as nothing), and `%v` prints the color value
  in a default form.

### 5.2 `app_test.go`

```go
package ui

import "testing"

func TestScreenTypeString(t *testing.T) {
    cases := []struct {
        name   string
        screen ScreenType
        want   string
    }{
        {name: "pods screen", screen: ScreenPods, want: "Pods"},
        {name: "deployments screen", screen: ScreenDeployments, want: "Deployments"},
        {name: "out-of-range value", screen: ScreenType(99), want: "Unknown"},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := tc.screen.String()
            if got != tc.want {
                t.Errorf("ScreenType(%d).String() = %q, want %q", tc.screen, got, tc.want)
            }
        })
    }
}
```

Two things worth calling out:

- **`tc.screen.String()`** — we're testing a **method** on a type, so we call it
  on the value (`tc.screen`), not as a free function. `ScreenType` is really just
  an `int` (see the `iota` enum in the main guide, section 4.2), so `String()` is
  what turns `ScreenPods` into the word `"Pods"` for the header.
- **`ScreenType(99)`** is a deliberate **out-of-range value**. There is no
  screen numbered 99, so this forces the `default:` branch to return `"Unknown"`.
  This is the single most valuable row in the table — see section 9.

---

## 6. Running the tests

From the project root:

```bash
go test ./...
```

`./...` means "this directory and every sub-package." Output on success is
refreshingly quiet — Go only reports the packages that *have* tests:

```
ok      yak8sui/pkg/ui   0.309s
?       yak8sui/pkg/k8s  [no test files]
?       yak8sui/cmd/yak8sui  [no test files]
```

For detail, add **`-v`** (verbose) to see every subtest by name:

```bash
go test ./pkg/ui/ -v
```

```
=== RUN   TestScreenTypeString
=== RUN   TestScreenTypeString/pods_screen
=== RUN   TestScreenTypeString/deployments_screen
=== RUN   TestScreenTypeString/out-of-range_value
--- PASS: TestScreenTypeString (0.00s)
    --- PASS: TestScreenTypeString/pods_screen (0.00s)
    --- PASS: TestScreenTypeString/deployments_screen (0.00s)
    --- PASS: TestScreenTypeString/out-of-range_value (0.00s)
=== RUN   TestStatusColor
    ... (six subtests) ...
--- PASS: TestStatusColor (0.00s)
PASS
ok      yak8sui/pkg/ui   0.309s
```

Notice how `t.Run` names become paths like `TestStatusColor/pending_is_yellow`
(spaces become underscores). That naming is what lets you run **one** case:

```bash
go test ./pkg/ui/ -v -run 'TestStatusColor/pending'
```

The `-run` flag takes a regular expression matched against the test path — handy
when you're iterating on a single failing scenario.

---

## 7. Reading a failure

Passing tests are boring; the real value shows up when something breaks. Suppose
someone "simplifies" `statusColor` and accidentally makes `Pending` fall through
to red. Running the test now prints:

```
=== RUN   TestStatusColor/pending_is_yellow
    colors_test.go:26: statusColor("Pending") = red, want yellow
--- FAIL: TestStatusColor/pending_is_yellow (0.00s)
...
FAIL
FAIL    yak8sui/pkg/ui   0.31s
```

Everything you need is on one line:

- **`colors_test.go:26`** — the exact file and line of the failing `t.Errorf`.
- **the named subtest** `TestStatusColor/pending_is_yellow` tells you *which
  scenario*.
- **`= red, want yellow`** — the `got, want` pattern spells out the actual vs
  expected value. This is why the order in `t.Errorf("... = %v, want %v", got, tc.want)`
  matters and is a near-universal Go convention: **actual first, expected
  after `want`**. Get it backwards and every failure message lies to you.

The other five rows still ran and still passed — one broken case doesn't hide
the rest. That isolation is a direct benefit of `t.Run`.

---

## 8. Why table-driven? The payoff

You could write six separate `TestStatusColorRunning`, `TestStatusColorPending`…
functions. The table-driven style wins for concrete reasons:

- **Adding a case is one line.** Kubernetes adds a new phase you care about?
  Append `{name: "...", status: "...", want: ...}` to the slice. No new function,
  no copy-pasted boilerplate.
- **The assertion logic exists once.** The `got != want` check and its error
  message live in a single place. Fix or improve the message once, every case
  benefits.
- **The cases read like a specification.** Anyone can scan the table and see, at
  a glance, the full contract: "Running and Succeeded are green, Pending is
  yellow, everything else is red." The test doubles as documentation.
- **Uniform, greppable failures.** Every case fails the same way, so output is
  predictable and the failing scenario is named.

This is *the* dominant testing pattern in the Go standard library itself — open
almost any `_test.go` in the Go source tree and you'll find a `cases := []struct`.

---

## 9. Choosing good cases (the `default` branch trap)

Writing the test is easy; writing *good* cases is the skill. A few principles
this small table already demonstrates:

- **Cover every branch.** `statusColor` has three outcomes (green/yellow/red);
  every one appears in the table. A `switch` with an untested branch is a bug
  waiting to happen.
- **Do not forget the `default`.** The easiest branch to leave untested is the
  catch-all `default:`. Both functions have one, and both are exercised —
  `statusColor("")` and `ScreenType(99)`. The `ScreenType(99)` case is
  especially instructive: in normal program flow that value can *never* occur
  (the only screens are `ScreenPods` and `ScreenDeployments`), yet the test
  reaches into that corner on purpose to prove the safety net works.
- **Include boundary/edge inputs.** The empty string `""` is the classic edge
  case for any function taking a `string`. It costs one line and catches a whole
  category of "what if it's blank?" bugs.
- **Give each row a descriptive `name`.** `"empty string is red"` tells you what
  broke without opening the file. `"case6"` does not.

> **A note on honesty:** these tests verify *current, documented behavior*. They
> don't prove the behavior is *desirable* — e.g. maybe one day you'd want an
> unknown pod phase to be orange, not red. That's fine: the test would then fail,
> you'd read the failure, and consciously update the `want`. A failing test after
> an intentional change is the test doing its job, not an obstacle.

---

## 10. The honest limitation: change-detector tests

Now the uncomfortable truth about `colors_test.go` specifically: **as a
bug-catching net, it is weak** — and it's worth understanding *why*, because this
is a trap you'll meet constantly.

Put the code and the test side by side:

```go
// colors.go
case "Running", "Succeeded": return tcell.ColorGreen
// colors_test.go
{status: "Running", want: tcell.ColorGreen},
```

The test is a **line-for-line mirror of the implementation**. It re-states the
exact mapping the `switch` already declares. There is no *independent* way of
arriving at the expected answer — the test's "oracle" is just a copy of the code.
A test like this is called a **change-detector test** (or a tautological test),
and it has two weaknesses:

- **It can't catch a logic mistake you'd actually make.** If you deliberately
  change the mapping, you edit the test to match — so it never asks "did you
  *mean* to do that?" It only fires on *accidental* edits to the function.
- **The code it guards is nearly bug-free already.** A `switch` returning
  constants is about the lowest-risk code in the repo, and a wrong color is a
  cosmetic defect you'd spot instantly on screen — not a silent corruption. Low
  bug probability × low severity = low insurance value.

So the value-to-existence ratio here is modest. The test is cheap (microseconds,
~zero maintenance), but *cheap is not the same as valuable*.

### The rule this teaches

A test earns its keep when its expected value is computed by a **different
route** than the code under test. If the test just echoes the implementation's
own logic back at it, you've written documentation, not a safety net.
`app_test.go` (`ScreenType.String()`) is in the same category — also a mirror of
a `switch`.

### So why write them at all here?

Two honest reasons, both about *this* project rather than production robustness:

1. **Learning.** These are the simplest possible pure functions, so the
   table-driven *mechanics* (the `[]struct`, `t.Run`, `got`/`want`) are visible
   with nothing else in the way. That is the point of this guide.
2. **Infrastructure.** They turn the CI `go test` step (section 11) from a no-op
   into something real, so the *next*, more valuable test has a home.

### What a test that actually earns its keep looks like

Contrast with the ready-ratio logic in `pkg/ui/deployments.go`:

```go
readyText := fmt.Sprintf("%d/%d", d.Available, d.Replicas)
```

Here a test is worth writing, because there is a **real, plausible bug**: the two
operands could be swapped (`%d/%d` with `Replicas, Available`). A test asserting
that `{Available: 3, Replicas: 5}` renders as `"3/5"` — an expectation a human
writes independently, knowing the intended order — would catch a mistake someone
would genuinely make. `colors_test.go` can never do that, because there is no
ordering, combination, or calculation to get wrong. See next-step #1 in section
12 for turning that logic into a test that pulls its weight.

---

## 11. How this plugs into CI

Open `.github/workflows/ci.yml` and you'll find this step was already there,
waiting:

```yaml
- name: Test
  run: go test ./...
```

Until now that step ran and found nothing to do. With these two files, **every
push and pull request now actually exercises the color and screen-name logic on
GitHub's servers.** If a future change breaks the contract in the table, the CI
check goes red *before* the code can merge — no cluster or terminal required,
because we tested the pure layer.

That is the quiet reward of the layered architecture from the main guide: the
logic that *can* be tested cheaply now *is*, automatically, on every change.

---

## 12. Where to go next

These two tests cover the pure functions. The natural progression, in rough
order of difficulty:

1. **Test more pure logic.** The deployments view computes a ready/total ratio
   and picks green-vs-red based on `Available < Replicas`. Extract that decision
   into a small pure helper (e.g. `deploymentReady(avail, total int32) bool`) and
   table-test it — a nice refactor-for-testability exercise.
2. **Introduce an interface to test impure code.** Functions like `k8s.ListPods`
   hit a real cluster, so they can't be unit-tested as-is. The standard Go answer
   is to define a small **interface** that the UI depends on, then pass a **fake**
   implementation in tests that returns canned `PodInfo` slices — no network. This
   is the "interfaces + dependency injection" concept the main guide hinted at.
3. **Try `t.Parallel()`** inside subtests to run independent cases concurrently —
   a gentle introduction to Go's concurrency story.
4. **Explore `go test -cover`** to see what percentage of your code the tests
   execute:
   ```bash
   go test ./... -cover
   ```

---

## 13. Resources

- [The `testing` package](https://pkg.go.dev/testing) — the full API for `*testing.T`, `t.Run`, `t.Errorf`, `t.Parallel`, and more.
- [Go by Example: Testing](https://gobyexample.com/testing-and-benchmarking) — a concise, runnable introduction.
- [Go Wiki: TableDrivenTests](https://go.dev/wiki/TableDrivenTests) — the canonical write-up of the pattern used here.
- [`go test` command docs](https://pkg.go.dev/cmd/go#hdr-Test_packages) — every flag, including `-run`, `-v`, and `-cover`.
- [The main yak8sui guide](guide_en.md) — architecture, `client-go`, and `tview` context for everything referenced above.
