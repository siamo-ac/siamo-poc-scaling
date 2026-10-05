# siamo-poc-scaling

POC 10 of the Siamo backend-architecture series: **a load balancer and a
shard router in Go** — what "scale out" actually looks like in motion.
Built to *show* distribution and sticky routing in a live demo.

## The concept (plain language)

**Vertical scaling** = making one server bigger (more CPU, more RAM).
Simple, but it hits a ceiling: one machine can only get so big, and if it
dies, everything dies.

**Horizontal scaling** = adding more identical servers ("replicas") and
spreading the work across them. No single ceiling, no single point of
failure — and you can add or remove servers as demand changes.

The catch: somebody has to decide *which* server gets each request. That's
the **load balancer**, and this POC demos two strategies:

1. **Round-robin** — requests go to backend 1, then 2, then 3, then back
   to 1. Dead simple, perfectly even.
2. **Shard routing (consistent hashing)** — requests carry a key (a user
   id, a tenant id), and the same key *always* lands on the same backend.
   "Sticky". This is how you keep per-user caches or sessions local to one
   machine — same idea as sharding in the data-management POC
   (`siamo-poc-data-management`), applied to routing instead of storage.

## Run the demo

Needs Go (1.21+). All commands from this directory.

**One-command guided demo:**

```bash
./scripts/demo.sh
```

**Manual run** (three terminals):

```bash
# Terminal 1-3: three identical backends
go run ./cmd/backend -addr 127.0.0.1:9001 -id backend-1
go run ./cmd/backend -addr 127.0.0.1:9002 -id backend-2
go run ./cmd/backend -addr 127.0.0.1:9003 -id backend-3

# Terminal 4: the balancer
go run ./cmd/balancer -addr 127.0.0.1:18080 \
  -backends "http://127.0.0.1:9001,http://127.0.0.1:9002,http://127.0.0.1:9003"
```

Then:

```bash
# Round-robin: watch X-Backend-Id rotate 1,2,3,1,2,3...
for i in 1 2 3 4 5 6; do curl -s -o /dev/null -D - http://127.0.0.1:18080/work | grep -i X-Backend-Id; done

# Sticky: key=alice always hits the same backend
curl -s "http://127.0.0.1:18080/route?key=alice"   # same backend every time
curl -s "http://127.0.0.1:18080/route?key=bob"     # maybe a different one — but stable

# The balancer's own tally
curl -s http://127.0.0.1:18080/status
```

## What to observe

- **Even distribution**: 12 requests through `/` give exactly 4 hits per
  backend (visible in `X-Backend-Id` headers and `/status`). That's
  round-robin doing its job.
- **Stickiness**: `?key=alice` returns the *same* `X-Backend-Id` on every
  call, while different keys spread across backends. Consistent hashing
  means adding a 4th backend only remaps ~1/4 of keys, not everything.
- **Identity, not magic**: each backend names itself in `X-Backend-Id`
  (header) and the JSON body, and the balancer adds `X-Routed-To`. You can
  see the decision, not just the result.

**Scaling out by hand:** start a 4th backend on port 9004 and restart the
balancer with 4 URLs — traffic immediately splits 4 ways. That's the whole
pitch of horizontal scaling: capacity is "add a box", not "buy a bigger
box".

## docker-compose.yml

Included to show the *shape* of a 3-replica deployment: one `balancer`
service, three `backend-N` replicas. The images are **placeholders** — no
Dockerfiles ship with this POC, so build your own images or, honestly, run
`scripts/demo.sh`, which is the real runnable demo.

## Honest limits (POC — not production hardening)

- **No real autoscaling.** Production uses Kubernetes HPA / cloud
  autoscaling groups driven by metrics. This POC scales when a human types
  a command.
- **No health checks on backends.** A dead backend still gets its turn in
  the rotation. Production balancers probe and eject failures.
- **No TLS, no auth** on the demo traffic — that's POC 9's topic, kept
  separate on purpose.
- **No session persistence / graceful drain** — restarting the balancer
  resets `/status` counters; in-flight requests during a restart are
  dropped.
- Round-robin ignores backend load (a slow backend gets the same share).
  Production often uses least-connections or weighted routing.
- Stdlib only (`net/http/httputil`), deliberately.
