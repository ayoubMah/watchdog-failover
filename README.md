# 🐶 Watchdog Demo

**Container health monitoring & failover without Kubernetes**

## What It Does

- Monitors multiple Spring Boot app instances for failures
- Auto-redirects traffic when containers crash
- Demonstrates core Kubernetes concepts manually

## Structure

```
victim-app/      # Buggy service (2 instances)
watchdog-app/    # Health monitor
docker-compose.yml
```

## How It Works

1. Watchdog checks `/status` on victim instances
2. If primary fails → traffic switches to backup
3. No manual intervention needed

## Run It

bash

```bash
docker-compose up --build
```

## Prerequisites

- Java 21, Maven, Docker

## Why This Matters

Learn what Kubernetes actually does by building it yourself:

- Health checks → liveness probes
- Failover → Service load balancing
- Manual logic → Declarative state