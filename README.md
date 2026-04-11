# Watchdog Demo

**Container health monitoring & automatic failover — no Kubernetes required**

A hands-on demonstration of how Kubernetes-style resilience works under the hood, rebuilt from scratch in Go.

---
## demo

https://github.com/user-attachments/assets/65eec45b-8bac-4a9c-a22f-a7b3867b9f01

---

## What It Does

- Runs `victim-a` on startup; `victim-b` is built but kept stopped (standby)
- A watchdog polls `victim-a` every 5 seconds via HTTP
- A built-in reverse proxy always serves on `:18080` — traffic switches automatically on failover, no manual port change needed
- If `victim-a` goes down, the watchdog starts `victim-b` and reroutes the proxy to it
- Once `victim-a` recovers, the proxy switches back to the primary automatically

---

## Stack

| Component | Tech |
|---|---|
| Victim app | Go 1.22, `net/http` |
| Watchdog app | Go 1.22, `net/http`, Docker CLI |
| Containers | Docker, Alpine 3.19 (multi-stage builds) |
| Orchestration | Docker Compose |

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                      Docker Network                      │
│                                                          │
│           ┌─────────────────────────────────┐           │
│  client   │           watchdog              │           │
│ ────────► │  :9998 proxy  │  :9999 status   │           │
│           └───────┬───────┴─────────────────┘           │
│                   │  routes to active backend            │
│          poll /status every 5s                           │
│                   │                                      │
│           ┌───────▼─────────┐   ┌─────────────────┐    │
│           │    victim-a     │   │    victim-b      │    │
│           │     :9995       │   │     :9995        │    │
│           │   (primary)     │   │  (standby/backup)│    │
│           └─────────────────┘   └─────────────────┘    │
└──────────────────────────────────────────────────────────┘
```

**Ports exposed on the host:**

| Service | Host port | Container port | Purpose |
|---|---|---|---|
| proxy (watchdog) | 18080 | 9998 | Always hit this — routes to active backend |
| victim-a | 18081 | 9995 | Direct access (debug/demo) |
| victim-b | 18082 | 9995 | Direct access (debug/demo) |
| watchdog status | 18083 | 9999 | Watchdog state endpoint |

---

## How It Works

1. `victim-a` and the watchdog start; `victim-b` is built but kept **stopped** (standby profile)
2. The proxy on `:9998` forwards all traffic to `victim-a` by default
3. The watchdog polls `victim-a` every 5 seconds
4. After 3 consecutive failures: starts `victim-b`, restarts `victim-a`, flips the proxy to `victim-b`
5. Once `victim-a` responds again: flips the proxy back to primary automatically
6. The watchdog `/status` endpoint always reflects current routing state

---

## Run It

```bash
docker compose up --build
```

To simulate a failure, stop `victim-a` from another terminal:

```bash
docker stop victim-a
```

The proxy switches automatically — keep hitting the same port:

```bash
# always use the proxy; it routes to whichever backend is active
curl http://localhost:18080/status

# check watchdog state
curl http://localhost:18083/status
```

---

## Project Structure

```
watchdog-go/
├── victim-app/
│   ├── main.go          # HTTP server, /status endpoint, heartbeat loop
│   ├── go.mod
│   └── Dockerfile       # Multi-stage Go build → Alpine
├── watchdog-app/
│   ├── main.go          # Health poll loop, docker start failover, /status endpoint
│   ├── go.mod
│   └── Dockerfile       # Multi-stage Go build → Alpine + docker-cli
└── docker-compose.yml   # Wires services + mounts /var/run/docker.sock
```

---

## Key Concepts Demonstrated

| What we built | Kubernetes equivalent | Note |
|---|---|---|
| HTTP poll on `/status` | Liveness probe | |
| Auto-start backup container | ReplicaSet self-healing | |
| Reverse proxy with auto-switch | Service load balancing | Proxy on `:18080` reroutes automatically on failover |
| Manual imperative logic | Declarative desired state | |

---

## Prerequisites

- Go 1.22+
- Docker & Docker Compose v2

---

## Configuration

The watchdog reads all tunables from environment variables. Defaults work out of the box with the provided `docker-compose.yml`.

| Variable | Default | Description |
|---|---|---|
| `PRIMARY_URL` | `http://victim-a:9995/status` | Endpoint to poll for liveness |
| `BACKUP_URL` | `http://victim-b:9995/status` | Endpoint to poll after failover |
| `PRIMARY_BACKEND` | `http://victim-a:9995` | Proxy target while primary is healthy |
| `BACKUP_BACKEND` | `http://victim-b:9995` | Proxy target after failover |
| `PRIMARY_CONTAINER` | `victim-a` | Container name the watchdog restarts on failover |
| `BACKUP_CONTAINER` | `victim-b` | Container name passed to `docker start` on failover |
| `CHECK_INTERVAL` | `5s` | Poll interval (any Go duration: `1s`, `500ms`, `1m`) |
| `FAILURE_THRESHOLD` | `3` | Consecutive failures before triggering failover |

Example override in `docker-compose.yml`:

```yaml
watchdog:
  environment:
    - CHECK_INTERVAL=2s
    - FAILURE_THRESHOLD=5
```

---

## Roadmap

| Priority | Feature | Why |
|---|---|---|
| 1 | ~~Configurable via env/flags~~ | ✅ Done |
| 2 | ~~Graceful shutdown~~ | ✅ Done |
| 3 | ~~Restart downed primary~~ | ✅ Done |
| 4 | ~~Automatic traffic switch~~ | ✅ Done |
| 5 | **Prometheus metrics** | Observability once core logic is solid |
| 6 | **Webhook / Slack alert** | Easy win after Prometheus |
| 7 | **Web UI dashboard** | Demo visuals, no functional value — do last |

---

## Future Features

- [ ] **Multiple failover targets** — maintain a pool of N backups and pick the next healthy one round-robin
- [ ] **Prometheus metrics endpoint** — expose `/metrics` with counters for checks, failures, and failovers
- [ ] **Web UI dashboard** — minimal HTML page served by the watchdog showing live status of all instances
- [ ] **Webhook / Slack alert** — fire an HTTP webhook or Slack notification when a failover occurs
- [ ] **docker compose watch** — hot-reload on code changes during local development without full rebuild
