# Architectural Blueprint
*Generated on: 2026-08-09*

## 1. Objective & System Changes

Update the checkout fetcher to optionally run as a continuous service via a `--service` flag, instead of as a one-shot cron job bounded by `--runtime`.

Key behavior:
- Service mode (`--service`): run continuously, ignoring `--runtime`. Cron mode keeps the existing `context.WithTimeout(ctx, runtime)` behavior.
- Exit via typical Linux signals: SIGTERM, SIGQUIT, plus existing Ctrl-C (os.Interrupt).
- SIGTERM/SIGQUIT do NOT immediately cancel the work context. Instead they trigger a graceful drain: stop scheduling new work (no new event dispatches, no new loop iterations), but let already in-flight HTTP fetches and DB writes complete using the live context.
- Hard stop: if the graceful drain has not completed within a 10-second (shutdownGracePeriod) grace period, force-exit via `os.Exit(1)`.
- Natural graceful drain completes with exit code 0 and runs existing `defer database.Close()`.

Implementation details:
- `eventCheckoutLoop` gains a `stopCh <-chan struct{}` parameter used to gate dispatch of new events in its `select` (alongside the existing `ctx.Done()` branch). Passing `nil` for `stopCh` leaves behavior unchanged (nil channel blocks forever in select).
- The main loop checks `stopCh` before starting a new iteration and in its bottom `select` to stop scheduling new iterations.
- The signal handler closes `stopCh` (no-op safe via `sync.Once`) and arms a `time.AfterFunc(shutdownGracePeriod, ...)` that logs and calls `os.Exit(1)`.

## 2. File Scope Tracker
- [ ] `internal/cmd/root.go` — add `--service` bool flag with `Sources: cli.NewValueSourceChain(cli.EnvVar("FETCH_CHECKOUTS_SERVICE"))`.
- [ ] `internal/cmd/checkoutsfetcher/cmd.go` — service-mode branch, stopCh-based drain signal handling, `eventCheckoutLoop` signature update.
- [ ] `internal/cmd/checkoutsfetcher/cmd_test.go` — update 6 `eventCheckoutLoop` call sites (lines ~81, 102, 162, 296, 410, 604) to pass `nil` for the new `stopCh` parameter.

## 3. Sequential Build Steps
1. Add `const shutdownGracePeriod = 10 * time.Second` in `internal/cmd/checkoutsfetcher/cmd.go`.
2. In `FetchCheckoutsV2`, branch on `serviceMode := cmd.Bool("service")`:
   - Service: `ctx, cancel = context.WithCancel(ctx)`; skip `--runtime` validation; log that `--runtime` is ignored.
   - Cron (default): keep existing `runtime <= 0` validation and `context.WithTimeout(ctx, runtime)`.
3. Replace the signal setup block (currently lines ~64-69): `signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)`; on first signal log a "draining, no new work" message, close `stopCh` via `stopOnce` (do NOT cancel ctx), and arm `time.AfterFunc(shutdownGracePeriod, ...)` that logs and calls `os.Exit(1)`.
4. Gate new work in the main loop: check `<-stopCh` (non-blocking) before each iteration and add `case <-stopCh:` to the bottom `select` alongside `ctx.Done()` and `time.After(interval)`.
5. Update `eventCheckoutLoop` signature to accept `stopCh <-chan struct{}`; add `case <-stopCh: break loop` to the dispatch `select` (keep `wg.Wait()` so in-flight `processEventCheckouts` finish with the live ctx); add a top-of-function early return when `stopCh` is already closed.
6. Add `--service` flag to the `checkout-fetcher` command in `internal/cmd/root.go`.
7. Update the 6 `eventCheckoutLoop` call sites in `internal/cmd/checkoutsfetcher/cmd_test.go` to pass `nil` as `stopCh`.

Verification:
- `gofmt` changed files; `go build ./...`
- `godotenv go test ./internal/cmd/checkoutsfetcher`
- Manual smoke: run `--service --interval 1s`, send SIGTERM -> in-flight drains then exit 0; simulate hang -> exit 1 after 10s.

---
*Context Anchor: Load this file into Build Mode via `@PLAN.md` to begin execution.*