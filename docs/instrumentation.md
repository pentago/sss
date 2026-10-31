# Profiling and tracing

sss uses Go's built-in tools only: `net/http/pprof` for profiles and `runtime/trace` for execution traces. You need a Go toolchain on the machine you analyse from.

## Enable

```sh
PPROF_LISTEN_ADDR=127.0.0.1:6060 sss
```

The endpoint is unauthenticated: keep it on localhost. In Docker, listen on all interfaces inside the container and publish the port only to the host's loopback:

```yaml
services:
  sss:
    environment:
      PPROF_LISTEN_ADDR: 0.0.0.0:6060
    ports:
      - "127.0.0.1:6060:6060"
```

Enabling it also samples 1 in 100 mutex contentions and about one blocking event per millisecond of blocked time, so the `mutex` and `block` profiles have data.

## Profiles: where CPU and memory go

```sh
go tool pprof -http=:8081 'http://127.0.0.1:6060/debug/pprof/profile?seconds=30'  # CPU while you use sss
go tool pprof -http=:8081 http://127.0.0.1:6060/debug/pprof/heap                  # live memory
go tool pprof -http=:8081 http://127.0.0.1:6060/debug/pprof/allocs                # allocations since start
```

Other profiles: `goroutine`, `mutex`, `block`. Use `-top` instead of `-http` for text output. The web UI has flame graph, top and source views. `go tool pprof` keeps a copy of every fetched profile in `~/pprof`; set `PPROF_TMPDIR=/tmp` to put them elsewhere.

## Traces: where one request's time goes

```sh
curl -o trace.out 'http://127.0.0.1:6060/debug/pprof/trace?seconds=10'  # reproduce the slow action meanwhile
go tool trace trace.out
```

Open **User-defined tasks** for these tasks:

| Task | Regions | Logs |
|---|---|---|
| `http`, one per request | none | `request`: method and path |
| `feed.refresh`, one per refresh | `fetch`, `read_body`, `parse`, `process`, `store`, `icon` | `feed_id` |

The task view shows each instance's duration and how much of it was spent running, blocked on network or locks, waiting in the scheduler, or in GC. To get a pprof-style view from the trace instead, use `go tool trace -pprof=net|sync|syscall|sched trace.out`.

## Adding instrumentation

```go
ctx, task := trace.NewTask(ctx, "name") // a unit of work, e.g. a job
defer task.End()
defer trace.StartRegion(ctx, "step").End() // a step inside it
```

These calls cost almost nothing while no trace is being recorded. Wrap `trace.Log` arguments that are expensive to build in `if trace.IsEnabled()`.
