# Fair-Dispatch — Reference Doc

**Event:** WeMakeDevs AWS "First Commit" (Sept 17–20, 2026), Build It track

## Project Description

A generic fair-dispatch engine for gig-economy matching problems — cabs, grocery delivery, food delivery — built on one core primitive: **atomic resource claiming under concurrent load, with fairness baked in.**

**The problem it solves:** whenever many concurrent requests compete for a small, scarce pool of resources (delivery riders, drivers, grocery pickers), naive systems either double-assign the same resource to two requests (a real bug class in smaller logistics platforms) or ignore fairness (some riders get slammed while others idle nearby). This engine guarantees exactly-one-winner claims under race conditions and weights assignment by fairness + proximity, not just "whoever's fastest to respond."

**Who it's for:** not a Swiggy/Zomato replacement — those are mature, ML-augmented systems built over years. This is aimed at smaller/regional delivery platforms, dark-store operators, or fleet aggregators who currently run on first-come-first-served logic or manual dispatch, and don't have the engineering resources to build a correctness-guaranteed fairness layer themselves. In the hackathon demo, it's framed as **a proven capability, not a finished product** — the hard primitive underneath any real dispatch system, demonstrated under genuine concurrent load.

**Why generic instead of single-vertical:** the matching problem is the same shape across cabs, grocery, and food delivery — only the resource type and constraints differ (a rider handles 1 order at a time, a grocery picker can juggle several, a cab has a destination constraint). Building the core once and swapping a config, rather than three separate apps, is both more honest about hackathon time constraints and a stronger "infra, not app" pitch.

## Stack

- **Language:** Go (persistent process, not Lambda — keeps the concurrency model real)
- **DynamoDB** (via LocalStack) — two tables:
  - `resources`: id, type, status (AVAILABLE/HELD/BUSY), fairness counters (orders_last_hour, idle_since), location
  - `requests`: id, domain, status, assigned_resource_id, created_at, held_until (native TTL)
  - Atomic claim via conditional `UpdateItem` (`status = :available`) — the correctness core
- **SQS** (via LocalStack) — `request-intake` queue; dispatcher workers pull from it, giving real backpressure under burst load
- **LocalStack + `awslocal` CLI** — emulates DynamoDB/SQS locally, no AWS account/billing needed (Build It track)
- Optional/cuttable: CloudWatch-equivalent metrics dashboard

**Track:** Build It, chosen after AWS account/deployment friction — doesn't affect eligibility for the top-10 Amazon interview fast-track, which is the actual priority over prize money.

## Flow

```
Request comes in (food order / grocery order / cab ride)
        ↓
   SQS intake queue  ←── gives backpressure under burst load
        ↓
  Dispatcher worker pulls request
        ↓
  Query available resources (riders/pickers/drivers) for this domain
        ↓
  Score candidates: proximity + fairness (idle time, recent load)
        ↓
  Atomically claim the top-scored resource
  (DynamoDB conditional write — status must still be AVAILABLE)
        ↓
   ┌─────────────┴─────────────┐
   │                           │
 Claim succeeds           Claim fails (lost the race)
   │                           │
 Resource → HELD           Retry with next-best candidate
 (TTL: e.g. 5 min to confirm)
   │
   ├─ Confirmed in time → BOOKED (permanent)
   └─ TTL expires        → resource auto-releases to AVAILABLE
```

**Domain configs** (`configs/food-delivery.yaml`, `grocery.yaml`, `cabs.yaml`) parametrize resource type, capacity per resource, fairness/proximity weighting, and hold TTL — swapping the config file is what proves the core is genuinely generic, not three separate codebases.

## Build Order

1. LocalStack + DynamoDB/SQS setup, Go SDK pointed at local endpoint
2. Atomic claim logic (`resources.go`) — verify with a two-goroutine race test before anything else
3. Matcher + fairness scoring, single domain first
4. SQS intake + worker wiring
5. Thin HTTP API layer
6. Swap in a second domain config (e.g. cabs) — proves genericness
7. Load-test script — concurrency-correctness proof for the demo video
8. Dashboard — only if time remains

## Demo Plan

- Fire many concurrent requests at a small resource pool; show zero double-claims, clean rejections/queuing instead of failures
- Show a TTL hold expiring and the resource releasing back to AVAILABLE
- Swap the domain config live (food → cabs) to prove the core is generic
- Frame explicitly: *"this isn't a full delivery platform — it's the dispatch-fairness core any of these systems needs, proven under real concurrent load."*