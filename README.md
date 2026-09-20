# ⚡ Fair-Dispatch Engine

> A generic, domain-agnostic dispatch engine for gig-economy platforms (cabs, food delivery, grocery) built on one core primitive: **atomic resource claiming under concurrent load, with fairness baked in.**

*Built for WeMakeDevs x AWS "First Commit" Hackathon (Sept 2026) // Build It Track*

## 🔥 The Problem

When many concurrent requests compete for a small pool of scarce resources (delivery riders, cab drivers, grocery pickers), naive systems either:

1. **Double-assign** the same resource to two requests (a real bug in smaller logistics platforms)
2. **Ignore fairness** i.e. some riders get slammed with back-to-back orders while others idle nearby

Fair-Dispatch solves both with a single engine.

## 🏗️ How It Works

```
  POST /dispatch { zone: "Salt Lake", domain: "cabs" }
              ↓
        SQS Intake Queue  ←── backpressure under burst load
              ↓
        Dispatcher Worker polls queue
              ↓
        Query DynamoDB GSI (ZoneStatusIndex)
        → Only AVAILABLE resources in the requested zone
              ↓
        Score candidates: fairness x proximity weights
              ↓
        Atomically claim top-scored resource
        (DynamoDB conditional write: status MUST be AVAILABLE)
              ↓
     ┌────────┴────────┐
     │                 │
  Claim succeeds    Claim fails (race lost)
     │                 │
  AVAILABLE → HELD   Retry next-best candidate
     │
  POST /confirm → HELD → BUSY
     │
  POST /complete → BUSY → AVAILABLE
                   (orders++ , idle_since = now)
```

## 🧠 Key Design Decisions

| Decision | Why |
|---|---|
| **DynamoDB Conditional Writes** | `ConditionExpression: status = AVAILABLE` guarantees exactly-one-winner under concurrent load. No distributed locks needed. |
| **Zone-Based GSI Sharding** | `ZoneStatusIndex` (hash: `zone`, range: `status`) replaces full table scans with O(1) targeted queries. Scales to millions of resources. |
| **SQS as Buffer** | Decouples request intake from processing. 50 concurrent requests don't overwhelm the worker, they queue gracefully. |
| **Event-Driven TTL** | Uses **SQS Delay Queues** for zero-latency, millisecond-perfect resource releasing. When a driver is HELD, a delayed message is pushed. When it appears, the worker does an O(1) conditional check to release it if still unconfirmed. No heavy database polling. |
| **YAML Domain Configs** | Swap `configs/cabs.yaml` for `configs/food_delivery.yaml` to change resource type and fairness/proximity weights. One engine, multiple verticals. |
| **Full State Machine** | `AVAILABLE → HELD → BUSY → AVAILABLE` with atomic transitions at every step. No resource ever gets stuck. |

## 📁 Project Structure

```
fair-dispatch/
├── cmd/
│   ├── api/          # 🌐 HTTP server (:8080) /dispatch, /confirm, /complete, /metrics
│   ├── worker/       # ⚙️ SQS polling loop: pulls requests, claims, and processes TTL events
│   ├── setup/        # 🛠️ Creates DynamoDB tables + SQS queues on LocalStack
│   └── loadtest/     # 🧪 Concurrent stress test with real-time fleet metrics
├── internal/
│   ├── api/          # HTTP handlers + metrics endpoint
│   ├── config/       # YAML domain config loader
│   ├── db/           # AWS client setup (DynamoDB + SQS)
│   ├── domain/       # Resource model, matcher/scorer, atomic claim logic
│   ├── intake/       # SQS publisher
│   └── worker/       # Dispatcher (poll → match → claim loop)
├── configs/
│   ├── cabs.yaml     # 🚕 Cab domain: 30% fairness, 70% proximity
│   └── food_delivery.yaml
├── .env.example      # Environment configuration template
└── README.md
```

## 🚀 Quick Start

### Prerequisites
- [Go 1.21+](https://go.dev/dl/)
- [LocalStack CLI](https://docs.localstack.cloud/getting-started/installation/) (`lstk`)

### 1. Start Infrastructure
```bash
lstk start
cp .env.example .env
go run cmd/setup/main.go
```

### 2. Start the Engine (two terminals)
```bash
# Terminal 1: API Server
go run cmd/api/main.go

# Terminal 2: Worker
go run cmd/worker/main.go
```

### 3. Run the Load Test
```bash
go run cmd/loadtest/main.go
```

## 📡 API Reference

| Endpoint | Method | Body | Description |
|---|---|---|---|
| `/dispatch` | POST | `{ "id": "req-1", "domain": "cabs", "zone": "Salt Lake" }` | Queue a dispatch request |
| `/confirm` | POST | `{ "resource_id": "cab-1" }` | Confirm a held resource (HELD → BUSY) |
| `/complete` | POST | `{ "resource_id": "cab-1" }` | Complete a job (BUSY → AVAILABLE, orders++) |
| `/metrics` | GET | | Fleet status counts (Available/Held/Busy) |

## 🔐 Environment Variables

| Variable | Default | Description |
|---|---|---|
| `AWS_ENDPOINT_URL` | `http://localhost:4566` | LocalStack endpoint |
| `QUEUE_URL` | | SQS queue URL (required) |
| `API_URL` | `http://localhost:8080` | API server URL (used by load test) |

## 🛠️ Tech Stack

- **Go** persistent worker process, real concurrency model
- **DynamoDB** (LocalStack) conditional writes for atomic claims, GSI for zone-sharded queries
- **SQS** (LocalStack) request buffering with backpressure
- **LocalStack** full AWS emulation, no cloud account needed

## 💡 What This Is (and Isn't)

This is **not** a Swiggy/Zomato replacement. Those are mature, ML-augmented systems built over years.

This is the **dispatch-fairness core** that any of these systems needs, the hard primitive underneath, proven under genuine concurrent load. It's aimed at smaller/regional delivery platforms, dark-store operators, or fleet aggregators who currently run on first-come-first-served logic and don't have the engineering resources to build a correctness-guaranteed fairness layer themselves.

---
