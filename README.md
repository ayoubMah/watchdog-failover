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
- If `victim-a` goes down, the watchdog starts the stopped `victim-b` via the Docker socket
- The watchdog exposes its own `/status` endpoint so you can observe state in real time

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
┌─────────────────────────────────────────────────┐
│                  Docker Network                 │
│                                                 │
│  ┌──────────┐      HTTP poll /status every 5s   │
│  │ watchdog │ ──────────────────────────────►   │
│  │  :9999   │                        ┌─────────┐│
│  │          │ ◄── UP / DOWN ──────── │victim-a ││
│  │          │                        │  :9995  ││
│  │          │ ── docker start ──►    └─────────┘│
│  │          │                        ┌─────────┐│
│  └──────────┘                        │victim-b ││
│                                      │  :9995  ││
│                                      └─────────┘│
└─────────────────────────────────────────────────┘
```

**Ports exposed on the host:**

| Service | Host port | Container port |
|---|---|---|
| victim-a | 18081 | 9995 |
| victim-b | 18082 | 9995 |
| watchdog | 18083 | 9999 |

---

## How It Works

1. `victim-a` and the watchdog start; `victim-b` is built but kept **stopped** (uses a Compose profile so it is not auto-started)
2. The watchdog polls `http://victim-a:9995/status` on a 5-second ticker
3. On failure (network error or timeout), watchdog runs `docker start victim-b` via the mounted Docker socket — this actually starts a stopped container
4. The watchdog's own `/status` endpoint reflects current state (`victim-a UP` or `victim-a DOWN, victim-b running`)
5. Each victim logs a heartbeat every second so you can watch liveness in `docker compose logs`

---

## Run It

```bash
docker compose up --build
```

To simulate a failure, stop `victim-a` from another terminal:

```bash
docker stop victim-a
```

Then watch the watchdog logs kick in and verify `victim-b` is running:

```bash
curl http://localhost:18083/status
curl http://localhost:18082/status
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
| Manual port switch on failure | Service load balancing | No proxy — you hit `:18082` manually after failover |
| Manual imperative logic | Declarative desired state | |

---

## Prerequisites

- Go 1.22+
- Docker & Docker Compose v2

---

## Future Features

- [ ] **Automatic traffic switch** — add a reverse proxy (nginx or a small Go proxy) in front so traffic shifts to `victim-b` without manual port change, making the K8s Service analogy real
- [ ] **Restart downed primary** — instead of only starting a backup, attempt to restart `victim-a` and switch traffic back once healthy (implements full reconciliation loop)
- [ ] **Multiple failover targets** — maintain a pool of N backups and pick the next healthy one round-robin
- [ ] **Configurable via env/flags** — make check interval, timeout, target URLs, and container names configurable without recompiling
- [ ] **Prometheus metrics endpoint** — expose `/metrics` with counters for checks, failures, and failovers
- [ ] **Graceful shutdown** — handle `SIGTERM` to flush state before the watchdog exits
- [ ] **Health history / state machine** — track consecutive failures before triggering failover (avoid flapping on transient errors)
- [ ] **Web UI dashboard** — minimal HTML page served by the watchdog showing live status of all instances
- [ ] **Webhook / Slack alert** — fire an HTTP webhook or Slack notification when a failover occurs
- [ ] **docker compose watch** — hot-reload on code changes during local development without full rebuild
