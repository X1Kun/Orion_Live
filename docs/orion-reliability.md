# Orion Reliability

## 1. Project Purpose

Orion-Live is a reliable Go backend for the interaction plane of a live-streaming product. It provides authenticated live sessions, persistent WebSocket chat, live reactions, gift-effect comments, replay metadata, replay feeds, and replay comments.

The system is designed to provide:

- Low-latency interaction for connected users
- Durable chat history with eventual persistence
- Reliable event distribution to real-time delivery, persistence, and analytics
- Strong consistency for limited gift-effect credits
- Bounded behavior during dependency failures and traffic bursts
- Automatic recovery after API, Worker, MySQL, Redis, or single RabbitMQ broker process restarts
- End-to-end observability across synchronous and asynchronous paths

Media ingestion, transcoding, live audio/video transmission, storage, and CDN delivery belong to the media plane and are outside this system.

## 2. Terminology

| Term | Meaning |
| --- | --- |
| Interaction plane | User interaction APIs and event processing surrounding media delivery. |
| Interaction event | A versioned record such as a chat message, live reaction, gift, or gift-effect comment. |
| Eventual consistency | Stores or views may temporarily disagree but converge asynchronously. |
| Idempotency key | Stable identity that makes repeated execution produce one logical effect. |
| User ID | The authenticated account identity obtained from the validated JWT. Clients do not choose or override it in message payloads. |
| Message ID | A UUIDv4 created by the client for one logical chat send. A retry reuses it; a new send creates a new one. `(live_session_id, user_id, message_id)` uniquely identifies a chat message; the ID has no ordering semantics. |
| Event ID | A stable identity for one event occurrence. For a chat event it is derived from `(event_type, live_session_id, user_id, message_id)` and is reused across publication and delivery retries. |
| Accepted at | The UTC server time captured once after admission checks and before initial publication. For chat events it is carried downstream unchanged as `occurred_at`; it provides approximate event time, not a global sequence. |
| History cursor | The auto-increment MySQL row ID assigned when a chat message is persisted. It supports stable history pagination and is not a per-client sequence or message identity. |
| RabbitMQ broker node | One RabbitMQ server process. Multiple API instances and multiple logical queues do not imply multiple broker nodes. |
| Durable queue | A queue definition that survives a broker restart. Its messages survive only when they are also published as persistent and broker storage is retained. |
| Quorum queue | One logical queue whose log is replicated across RabbitMQ cluster nodes and committed by a majority. A single-member quorum queue provides no node-level failover. |
| Publisher confirm | RabbitMQ acknowledgement that the broker accepted responsibility for a publication. |
| Mandatory routing | RabbitMQ returns a publication that cannot be routed to any queue; it proves at-least-one-queue routing, not delivery to every intended queue. |
| Consumer acknowledgement | Confirmation sent only after a Consumer has completed its required processing. |
| Transactional outbox | A database pattern that commits business data and a pending event in one local transaction. |
| Inbox | A Consumer-side record keyed by `(consumer_name, event_id)` that prevents duplicate effects while allowing independent Consumers to process the same event. |
| Poison message | Input that repeatedly fails for a deterministic reason. |
| Dead-letter queue | Queue that isolates messages after their retry budget is exhausted. |
| Bounded degradation | Partial fallback with explicit limits that protect downstream dependencies. |
| Fail closed | Reject an operation when a dependency required for correctness or traffic safety is unavailable. |
| Token bucket | A rate limiter that replenishes tokens at a fixed rate and permits a bounded burst up to the bucket capacity. |
| Live reaction | A transient append-only interaction of type `LIKE`, `HEART`, `CLAP`, or `FIRE`; it is broadcast immediately and stored only as an aggregate. |
| Gift-effect grant | A time-bounded grant that allows the gift sender to publish a fixed number of visually enhanced comments. |
| Replay metadata | Business metadata for a recorded live session, including title, cover, playback URL, duration, availability, and source session. Media bytes remain outside Orion-Live. |

## 3. System Features

| Feature | Behavior |
| --- | --- |
| Authentication | Register, log in, validate JWTs, and authorize protected HTTP and WebSocket endpoints. |
| Live sessions | Create and manage a live session with host, title, cover, status, and start/end timestamps. |
| Persistent WebSocket chat | Validate and rate-limit chat, publish it reliably, broadcast it to the room, and persist it asynchronously. |
| Recent room context | Let joining users read recent messages and reconnecting users recover messages missed during a short network gap. |
| Live reactions | Let viewers send `LIKE`, `HEART`, `CLAP`, or `FIRE` reactions for real-time animation and per-type aggregation. |
| Gifts | Accept a trusted gift result and create a time-bounded gift-effect grant. Payment processing remains external. |
| Gift-effect comments | Let the gift sender use a fixed number of visual-effect comments through an atomic MySQL transaction. |
| Interaction analytics | Maintain eventually consistent per-session and per-minute aggregates without blocking user requests. |
| Live interaction insights | Expose chat, reaction-by-type, gift, and peak-activity aggregates through a read API for hosts and operations. |
| Replay metadata | Create processing metadata when a live session ends, accept a trusted final media result, and expose available playback metadata without storing or transcoding media. |
| Replay feed | Return paginated replay metadata for completed live sessions. |
| Replay comments | Persist comments synchronously when the client requires an immediate database ID. |

## 4. System Architecture

```text
HTTP / WebSocket Clients
           │
           ▼
┌───────────────────────────┐       rate limits / cache       ┌───────┐
│        API Server         │◀───────────────────────────────▶│ Redis │
│ auth / live / replay / WS │                                └───────┘
└────────┬─────────┬────────┘
         │         │
         │         └── synchronous business transaction ──▶ MySQL
         │                                                       │
         │ direct chat/reaction publication                      │ pending Outbox events
         │                                                       ▼
         │                                                Outbox Publisher
         │                                                       │
         └───────────────────────┬───────────────────────────────┘
                                 ▼
                    ┌────────────────────────┐
                    │   RabbitMQ Interaction │
                    │        Exchange        │
                    └───────┬───────┬────────┘
                            │       │
             ┌──────────────┘       └─────────────────┐
             ▼                                        ▼
    per-API realtime queue                    Persistence Queue
             │                                  │           │
             ▼                                  ▼           └─▶ Persistence Retry ─▶ Persistence DLQ
    Realtime Subscriber                 Persistence Consumer
             │                                  │
             ▼                                  ▼
    WebSocket Hub / Rooms                      MySQL

                    Exchange ──▶ Analytics Queue
                                      │          │
                                      ▼          └─▶ Analytics Retry ─▶ Analytics DLQ
                              Analytics Consumer ──▶ MySQL

All processes ──▶ Prometheus metrics + structured logs
```

### Component responsibilities

| Component | Responsibility |
| --- | --- |
| API Server | Authentication, validation, traffic admission, synchronous business transactions, WebSocket lifecycle, and direct chat/reaction publication. |
| WebSocket Hub | Own in-memory rooms and deliver typed interaction events to locally connected clients. |
| Topology Initializer and Auditor | Declare shared RabbitMQ topology with separate privileges, verify required bindings, and close affected send gates when an invariant fails. |
| Outbox Publisher | Lease committed events, publish with confirms, and use a fenced claim token for state changes and bounded retry. |
| Persistence Consumer | Persist the first committed Chat value, suppress equal duplicates, and audit conflicting payload hashes without blocking WebSocket ingestion. |
| Analytics Consumer | Update business aggregates idempotently without blocking interaction requests. |
| MySQL | Source of truth for users, sessions, replay metadata, chat history, gift grants, gift-effect comments, outbox events, inbox records, and interaction aggregates. |
| Redis | Distributed rate limiting, read cache, and request coalescing support. It is not authoritative for gift credits. |
| RabbitMQ | On the baseline single broker node, buffer and route interaction events to independent real-time, persistence, and analytics paths with acknowledgement and isolated failure handling. |
| External gift system | Produces a trusted gift result; payment and financial correctness are outside Orion-Live. |
| External media plane | Produces the recording and playback URL after a live session ends. |
| Prometheus and structured logs | Operational metrics, health signals, correlation, and failure diagnosis. Prometheus is not the business analytics store. |

## 5. Key Design

### 5.1 Interaction event contract and RabbitMQ topology

Every event uses a versioned envelope:

```text
event_id
event_type
schema_version
correlation_id
idempotency_key
request_hash
user_id
live_session_id
occurred_at
payload
```

The topic exchange routes events by type:

```text
orion.interaction.events
├─ chat.message.accepted
├─ reaction.created
├─ gift.sent
├─ live_session.ended
└─ gift_effect_comment.created
```

The baseline deployment uses one RabbitMQ broker node even when multiple API instances are running:

```text
one RabbitMQ broker node
└─ orion.interaction.events
   ├─ one exclusive, auto-delete real-time queue per API instance
   ├─ one shared durable Persistence Queue
   │  └─ dedicated Persistence Retry Queue and Persistence DLQ
   └─ one shared durable Analytics Queue
      └─ dedicated Analytics Retry Queue and Analytics DLQ
```

- API instance count, logical queue count, and RabbitMQ broker-node count are independent deployment dimensions.
- Each API instance creates its own real-time queue and broadcasts matching events only to WebSocket clients connected to that instance. This queue is best effort and has no retry queue or DLQ.
- Persistence Consumer replicas compete on the shared Persistence Queue; Analytics Consumer replicas compete independently on the shared Analytics Queue.
- A topology initializer uses separate credentials with shared-topology `configure` permission to declare the exchange, durable queues, retry queues, DLQs, and bindings idempotently. Runtime publishers and durable Consumers cannot modify shared topology.
- Each API instance may configure only its own namespaced real-time queue and has publish permission plus read access to that queue; durable Consumers have read access only to their assigned shared queues.
- Persistence and Analytics use separate retry queues and DLQs because they have different retry policies, operational impact, and redrive procedures.
- The baseline uses durable classic queues, persistent messages, publisher confirms, and retained broker storage. This survives a RabbitMQ process or container restart, but not permanent loss of the broker host or disk.
- A future three-node RabbitMQ cluster may migrate durable processing queues to Quorum Queues for broker-node failover. A single-node deployment must not claim quorum-based high availability.
- Messages use stable IDs, persistent delivery, publisher confirms, and mandatory routing checks.
- Mandatory routing proves only that an event reached at least one queue; Publisher Confirm and mandatory routing do not independently prove delivery to the Persistence Queue.
- Startup and periodic topology audits validate required queues and bindings through access-controlled management credentials. A missing required binding triggers an alert, marks messaging readiness unhealthy, and closes the Chat/Reaction send gate until the topology is restored.
- Consumers acknowledge only after their local database transaction commits.
- Retryable Persistence and Analytics failures enter their own delayed retry queue with a bounded attempt count.
- Deterministic failures and exhausted messages enter the DLQ belonging to that processing path.
- Consumers use a unique business key or Inbox record to prevent duplicate effects.
- Reconnection recreates channels, QoS, publishers, and Consumers with exponential backoff and jitter. The topology initializer restores shared topology; runtime processes only validate it and recreate their permitted per-instance real-time queues.

### 5.2 Ordinary chat pipeline

```text
WebSocket message with a client-generated `message_id`
→ authenticate the connection and obtain `user_id` from its JWT
→ validate input and apply Redis rate limits
→ capture `accepted_at = time.Now().UTC()` once
→ derive a stable `event_id` from (`chat.message.accepted`, `live_session_id`, `user_id`, `message_id`)
→ compute `request_hash` from the canonical message payload
→ publish `chat.message.accepted` with `occurred_at = accepted_at`
→ verify routing and wait for Publisher Confirm
→ return `chat.ack` containing `message_id` and status `accepted`
→ Realtime Subscriber broadcasts to online clients
→ Persistence Consumer inserts the message idempotently
→ Analytics Consumer updates aggregates idempotently
```

- The client creates a new UUIDv4 `message_id` for each deliberate send and reuses the same ID only when retrying that send.
- The API never accepts `user_id` from the chat payload. It obtains `user_id` from the authenticated WebSocket connection.
- `(live_session_id, user_id, message_id)` is the chat business key and has a MySQL unique constraint. The session scope prevents accidental ID reuse in one room from conflicting with another room.
- Ordinary Chat is published directly to RabbitMQ and does not use an API-side Outbox or synchronous MySQL idempotency reservation. Its latency and buffering model intentionally differs from transactional Gift-effect comments.
- Chat `request_hash` covers canonical client-controlled meaning such as `live_session_id` and content; it excludes retry-varying server metadata such as `accepted_at`, `correlation_id`, and publication attempt details.
- Repeated publication of the same logical send produces the same `event_id`, allowing real-time clients and durable Consumer Inboxes to suppress duplicate effects.
- The Persistence Consumer stores `request_hash` with the message. The first `(live_session_id, user_id, message_id)` value committed to MySQL becomes authoritative; the system does not promise which of two concurrently accepted conflicting payloads wins.
- The same business key and hash is a normal duplicate and is acknowledged without another effect. The same key with a different hash is a protocol violation: the Consumer preserves the stored message, records a conflict audit and metric, and acknowledges the conflict without sending it through an automatic redrive loop.
- Real-time copies are provisional. Concurrent invalid payloads may be observed in different orders across per-API real-time queues; subsequent history reconciliation replaces them with the authoritative MySQL value. A second Chat ACK never means that an existing message was edited.
- `message_id` identifies a message but never determines message order. UUIDv7, Redis `INCR`, and a central sequence service are unnecessary for this consistency model.
- The API creates one immutable event for a publication attempt. Publisher retries reuse its original `event_id` and `accepted_at` instead of calling the clock again.
- Realtime, Persistence, and Analytics Consumers preserve the source `occurred_at`. They may record separate `processed_at` or `persisted_at` timestamps for latency measurement but never replace the event time.
- A duplicate client request may reach another API instance after an acknowledgement is lost. The first event persisted for the business key defines the stored `accepted_at`; duplicate events cannot overwrite it.
- RabbitMQ acceptance is the asynchronous Chat acceptance boundary only while the access-controlled topology has passed its required-binding checks. This guarantee is conditional on that deployment invariant.
- If reliable publication cannot be confirmed, the server reports failure and does not claim that the message was accepted.
- Persistence and analytics failures do not block WebSocket ingestion; they use bounded retry and DLQ handling.
- `chat_messages.id` is an auto-increment history cursor created only when MySQL persistence completes; it is separate from the client-generated `message_id`.
- History queries use `WHERE live_session_id = ? AND id > ? ORDER BY id LIMIT ?` and return the last row ID as `next_cursor`. The client keeps this cursor locally; the server does not maintain one sequence per client.
- Joining clients fetch recent room context. Reconnecting clients query after their last cursor and merge results by `(live_session_id, user_id, message_id)` so that WebSocket and HTTP copies are displayed once.
- Because real-time and persistence queues are consumed independently, an immediate history query may not contain a message already seen in real time. During a default five-second recovery window, the client repeats `after_cursor` queries after approximately 250 ms, 500 ms, one second, and two seconds, and merges results by business key.
- Without a persistence watermark, a finite reconnect procedure cannot prove that it has obtained every accepted message. Messages persisted after that window are eventually discoverable on a later history refresh or room entry; the system does not claim bounded-complete recovery.
- Clients display approximate chronological order using `accepted_at`, with `(live_session_id, user_id, message_id)` as a deterministic tie-breaker. Cursor order is stable MySQL insertion order, not strict send-time order across API instances.
- Real-time delivery remains best effort: disconnected or slow clients may miss the live broadcast even though the message is persisted later.

### 5.3 Live reactions

```text
WebSocket reaction (`LIKE`, `HEART`, `CLAP`, or `FIRE`)
→ validate the type and apply Redis user/room rate limits
→ publish `reaction.created`
→ Realtime Subscriber broadcasts the visual effect
→ Analytics Consumer increments the matching minute/type bucket
```

- Each click creates a new append-only event rather than mutating a per-user reaction state.
- Reaction uses deliberately non-idempotent client-intent semantics: every admitted WebSocket frame is a new Reaction intent and a successful publication creates one event. There is no client `reaction_id`, application-level ACK, or automatic client retry.
- TCP/WebSocket transport retransmission is transparent to the application. A client that sends another application frame, whether through another click, a bug, or abuse, creates another Reaction and is controlled by user/room Token Buckets.
- The API generates a fresh `event_id` for each received frame. Retries of that frame's RabbitMQ publication reuse the same `event_id`, so broker redelivery remains idempotent in the Analytics Inbox.
- Raw reaction events are not stored as permanent MySQL history.
- MySQL stores aggregates keyed by `(live_session_id, minute_bucket, reaction_type)`.
- The Analytics Consumer uses the shared Inbox policy in Section 5.5 to deduplicate RabbitMQ redelivery; this does not change the non-retrying client-intent semantics above.
- Real-time animation is best effort; aggregate statistics are eventually consistent.

### 5.4 Gift-effect comments

A trusted gift result creates a grant for the gift sender:

```text
GiftEffectGrant
├─ grant_id
├─ provider_gift_id
├─ provider_request_hash
├─ live_session_id
├─ user_id
├─ total_credits
├─ remaining_credits
├─ expires_at
└─ status
```

Each confirmed gift creates one grant. The user sees an aggregate balance, while consumption selects the active grant with the earliest `(expires_at, grant_id)`:

```text
Request with client-generated `message_id`
→ MySQL transaction
   ├─ return the original comment for an identical `(live_session_id, user_id, message_id)` retry
   ├─ return 409 when the same key is reused with a different request hash
   ├─ lock the LiveSession and require status `LIVE`
   ├─ select the earliest-expiring eligible grant using database UTC time and `SELECT ... FOR UPDATE`
   ├─ atomically decrement `remaining_credits`
   ├─ insert the gift-effect comment
   └─ insert `gift_effect_comment.created` into the Outbox
→ commit
→ return the comment and updated credit summary
→ Outbox Publisher sends the event
→ real-time delivery and analytics process it independently
```

- `provider_gift_id` is unique. An identical notification hash returns the existing result; reuse with a different user, session, gift, or credit payload returns `409`, leaves the Grant unchanged, and raises a security alert.
- Grant creation and its `gift.sent` Outbox event commit in one transaction.
- The credit read API returns `available_credits`, `next_expires_at`, and `next_expiring_credits` from indexed active grants. A separate balance table or Redis balance cache is added only if measurements justify it.
- Credit summaries are snapshots near transaction commit. Concurrent requests may immediately consume more credits, so the API does not promise that a returned balance remains unchanged.
- Gift-effect request hashes cover canonical behavior-changing input such as `live_session_id`, comment content, and a client-selectable effect type. A server-derived effect is excluded from the request payload and hash.
- Gift-effect comments use the same session-scoped `(live_session_id, user_id, message_id)` business identity as ordinary Chat, while retaining their stronger synchronous transaction semantics.
- Grant eligibility includes `status = 'ACTIVE'`, `remaining_credits > 0`, and `expires_at > UTC_TIMESTAMP(6)` in the locking query. The selection index is `(user_id, live_session_id, status, expires_at, grant_id)`.
- New Gift-effect mutations lock `LiveSession` before Grant rows. The ending transaction follows the same order, giving Gift comments a strict cutoff: either the comment commits while the session is `LIVE`, or it observes `ENDED` and consumes no credit.
- Deadlocks and lock-wait timeouts retry the complete transaction at most three times with jitter while preserving the same `(live_session_id, user_id, message_id)`.
- MySQL is authoritative for the grant; Redis does not reserve or compensate credits.
- Credit consumption, comment creation, and Outbox insertion succeed or fail together.
- The API returns success only after the MySQL transaction commits.
- A RabbitMQ outage delays downstream broadcast and analytics but cannot erase a committed comment.
- Payment processing, refunds, and financial ledger correctness remain outside this project.

### 5.5 Transactional Outbox and Consumer Inbox

- The API writes business data and a `PENDING` Outbox event in one MySQL transaction.
- Outbox records include `status`, `available_at`, `claimed_by`, a unique per-claim `claim_token`, `lease_until`, `attempt_count`, `last_error`, `published_at`, and timestamps.
- Publisher instances claim eligible records with a bounded lease and commit that claim before network publication; they never hold a database transaction open while waiting for RabbitMQ.
- Publisher Confirm changes an event to `PUBLISHED` only through a fencing update matching both `event_id` and the current `claim_token`. An expired Publisher cannot overwrite a newer owner's state.
- A lease that expires before a successful state update makes the event eligible for another claim. Both Publishers may publish, so the design remains at-least-once and relies on Consumer Inbox deduplication.
- Retryable publication errors use exponential backoff with a default automatic budget of 20 attempts and 24 hours. Deterministic schema or routing errors, and exhausted events, enter `FAILED` instead of retrying forever.
- Online controlled republish is permitted only while the original event is at most seven days old. This bound remains shorter than the 14-day Inbox retention window, covering cases where RabbitMQ processed an event but its Confirm or Outbox state update was lost.
- `PUBLISHED` rows remain in the hot table for 14 days before bounded-batch deletion or archival. `FAILED` rows remain for 30 days, but after the seven-day online-republish limit they are diagnosis records only and require offline inspection or data correction rather than publication to online Consumers.
- Outbox monitoring uses pending count, oldest pending age, failure count, and publish latency. An oldest pending age above one minute alerts; above five minutes closes admission for new operations that require an Outbox, returning `503` before their transaction starts.
- A crash after RabbitMQ acceptance but before the status update may publish the same `event_id` again.
- Durable Consumers share an Inbox table with `UNIQUE(consumer_name, event_id)`, allowing Persistence and Analytics to process the same event independently.
- A Consumer inserts its Inbox record in the same transaction as its business changes. A duplicate key skips repeated business work and is acknowledged safely.
- For Chat persistence, the stored message and Inbox metadata retain `request_hash`; a duplicate hash is skipped, while a different hash for the same logical identity follows the conflict-audit rule in Section 5.2.
- Automatic broker retries are bounded to at most one hour. Supported DLQ redrive and Outbox online-republish age are each at most seven days, and Inbox records are retained for 14 days, which includes the maximum retry or republish window plus a safety margin.
- DLQ redrive preserves the original `event_id`, `occurred_at`, idempotency key, and business identifiers. Redrive never creates a logically new event identity.
- Events older than the supported seven-day redrive window are not automatically returned to online Consumers. They require offline inspection and a controlled data correction.
- Inbox cleanup deletes expired records in bounded batches. The retention rule is `inbox_retention >= max(maximum_outbox_online_republish_age, maximum_broker_retry_duration + maximum_supported_dlq_redrive_age) + safety_margin`.

The system therefore provides at-least-once delivery with one logical database effect, not exactly-once delivery.

### 5.6 Live-session and replay lifecycle

```text
LiveSession(SCHEDULED)
→ LiveSession(LIVE)
→ LiveSession(ENDED)

LiveSession(ENDED)
→ create Replay(PROCESSING)
→ trusted media result
   ├─ Replay(AVAILABLE)
   └─ Replay(FAILED)
```

- `LiveSession` is the persistent business record for one broadcast. Its server-generated `live_session_id` is created once and remains unchanged through `SCHEDULED`, `LIVE`, and `ENDED`.
- A WebSocket Room is ephemeral per API instance and is keyed by the same `live_session_id`; it is not another `LiveSession` and does not own a separate business ID.
- Ending a session uses a MySQL transaction to change `LIVE → ENDED`, record database-UTC `ended_at`, and insert a `live_session.ended` Outbox event. The event tells API instances to close their local send gates and later remove Rooms after connections close.
- Ordinary Chat and Reaction use a weak cutoff because their high-volume path does not lock the LiveSession row. Each API rejects new frames as soon as it observes `ENDED`, but propagation delay may allow a small number of tail events after the authoritative `ended_at`.
- Any tail event already acknowledged by the API is still broadcast and persisted; the system never retracts an accepted interaction. Insights may exclude events with `accepted_at > ended_at` from official live-period aggregates while retaining them for audit and history.
- Gift-effect comments use the strict transactional cutoff defined in Section 5.4. Enforcing a zero-tail cutoff for every Chat and Reaction would require synchronous coordination on the hot path and is intentionally outside the baseline.
- Ending drains already accepted work and removes local Rooms after their connections close. The `LiveSession`, chat history, tail events, and analytics remain stored.
- Only an `ENDED` session may create a Replay. Creation is idempotent through `UNIQUE(source_live_session_id)`.
- `Replay` receives its own server-generated `replay_id` and retains `source_live_session_id`; it is a separate resource and never replaces or renames the original `LiveSession`.
- Orion creates the Replay in `PROCESSING`; the external media plane performs recording, storage, and delivery work outside this system and reports only a final `AVAILABLE` or `FAILED` result.
- A trusted media result includes a unique `provider_event_id`. Duplicate event IDs return the existing result without changing state.
- Valid baseline transitions are `PROCESSING → AVAILABLE`, `PROCESSING → FAILED`, and a compensating `FAILED → AVAILABLE`. `AVAILABLE` is terminal and cannot be downgraded by late `PROCESSING` or `FAILED` events.
- Automatic `FAILED → PROCESSING` retry is not part of the baseline. A future explicit, authenticated retry operation may introduce a new processing attempt.
- Media result endpoints require service authentication and replay protection. An `AVAILABLE` result must provide a playback URL from an approved media/CDN origin.
- The existing generic `Video` model is migrated to `Replay` rather than expanded into a media service.
- Only `AVAILABLE` replays appear in the public feed.
- Media upload, transcoding, storage, and CDN delivery remain external responsibilities.

### 5.7 Cache, admission control, and degradation

- MySQL is authoritative for durable business data.
- MySQL is selected for local ACID transactions, row locks, unique constraints, relational queries, and the Grant/Comment/Outbox atomic boundary. RabbitMQ buffering and reaction aggregation keep its expected workload within the project target.
- A NoSQL chat-history store is considered only after measured MySQL write or retention limits require horizontal partitioning; it would complement rather than replace MySQL transaction data.
- Under normal operation, Redis Lua scripts atomically enforce distributed Token Buckets keyed by user and room before Chat or Reaction publication.
- The Redis limiter uses a short operation deadline. A timeout or consecutive failures open the limiter circuit and enter `DEGRADED_LOCAL` instead of blocking WebSocket goroutines.
- In `DEGRADED_LOCAL`, each API instance uses bounded in-memory Token Buckets with TTL eviction and a bounded key count.
- A local per-user bucket uses 20% of the normal user refill rate and burst capacity because an established WebSocket is owned by one API instance; per-user connection limits reduce multi-connection bypass.
- The cluster-wide degraded room budget is 20% of the normal global room limit. Each instance receives `degraded_room_budget / MAX_API_REPLICAS`, so the aggregate remains bounded even when all allowed replicas are active.
- `MAX_API_REPLICAS` must match the deployment autoscaling cap and is validated as a positive startup setting. The baseline two-API deployment uses `MAX_API_REPLICAS = 2`; changing the deployment cap requires changing this configuration.
- Local degradation lasts at most 30 seconds. A successful Redis recovery probe returns the process to `HEALTHY`; otherwise it enters `FAIL_CLOSED`.
- In `FAIL_CLOSED`, new Chat and Reaction requests return `503`, while established WebSocket connections remain available for receiving events and read-only operations continue.
- Exceeding either a normal Redis limit or a degraded local limit returns `429`; `503` is reserved for dependency-driven refusal after the degradation budget expires.
- Cache misses use request coalescing to prevent concurrent reads for the same hot key.
- Redis read-cache failures use a MySQL fallback protected by deadlines, concurrency limits, and a circuit breaker.
- Fallback saturation returns a controlled `503` rather than overloading MySQL.
- Gift credits never use the Redis limiter or cache as their source of truth and remain protected by MySQL transactions during a Redis outage.
- Database writes invalidate or refresh related cache entries.

### 5.8 WebSocket lifecycle and broadcast

- WebSocket upgrades require authentication and live-room authorization.
- One ownership model controls the room map and prevents concurrent mutation.
- Each room has a bounded broadcast queue; each client has a bounded send queue.
- Slow consumers are disconnected rather than blocking the room.
- Read limits, write deadlines, and ping/pong heartbeats detect unhealthy connections.
- Typed event envelopes allow ordinary chat, live reactions, gifts, and gift-effect comments to share one broadcast path.
- `BroadcastIfPresent` never creates an empty room merely to deliver an event.
- Clients deduplicate repeated real-time events by `event_id`; chat history and real-time chat are merged by `(live_session_id, user_id, message_id)`.
- Join, leave, room deletion, reconnect, and shutdown follow explicit ordering.

### 5.9 Analytics and observability

- The Analytics Consumer maintains per-session and per-minute interaction aggregates in MySQL.
- Business aggregates include chat volume, reactions by type, gift activity, and gift-effect comment usage.
- `GET /live-sessions/:id/insights` returns aggregate series, totals, and peak-activity time without requiring a dedicated frontend.
- Prometheus records operational behavior such as request latency, publish latency, queue depth, retry count, DLQ depth, Consumer backlog, processing errors, and WebSocket connections.
- Redis limiter state, local-fallback admissions, fail-closed rejections, cache-fallback concurrency, and recovery time are exposed as bounded-cardinality metrics.
- Required-binding health, Chat payload conflicts, Outbox claim/fencing failures, oldest pending age, and tail events accepted after session end are monitored and alerted.
- High-cardinality business identifiers such as `user_id`, `message_id`, and `event_id` are excluded from Prometheus labels.
- `context.Context` propagates deadlines through HTTP, services, repositories, Redis, MySQL, and RabbitMQ operations.
- Correlation IDs connect API logs, event headers, Worker logs, and database operations.
- API and Worker processes implement graceful shutdown with a fixed deadline.
- `/healthz` reports process liveness; `/readyz` reports ability to accept traffic.

### 5.10 Security and public errors

- Required secrets and dependency configuration are validated before startup.
- Request bodies, pagination, WebSocket frames, connection counts, and message rates are bounded.
- RabbitMQ topology, publisher, and Consumer identities use separate least-privilege credentials; runtime identities cannot mutate shared durable topology.
- Trusted gift and media results require service authentication, unique provider event identities, bounded timestamp replay protection, and payload-hash conflict detection.
- Internal failures use typed errors and map to a stable public error envelope.
- Database errors, infrastructure details, stack traces, and credentials are never returned to clients.

## 6. Consistency and Reliability

### Consistency model

| Interaction | Guarantee | Design |
| --- | --- | --- |
| Ordinary chat acceptance | Conditional durable asynchronous acceptance | Success requires a confirmed routable publication while the access-controlled topology and required Persistence binding are validated; Confirm and mandatory alone do not prove that binding. |
| Ordinary chat persistence | Eventual consistency | MySQL preserves the first persisted `(live_session_id, user_id, message_id)` value; equal hashes are duplicates and conflicting hashes are audited without a defined winner. |
| Real-time room delivery | Best effort | Online copies are provisional; reconnecting clients eventually reconcile to persisted history, but a finite recovery window cannot prove completeness or strict cross-instance order. |
| Live reactions | Best-effort client intent and eventual aggregates | Every received frame is a new Reaction with no client ACK/retry; internal publication retry reuses `event_id`, and Analytics maintains per-minute counts. |
| Gift-effect comment | Read-after-write for the sender | MySQL atomically consumes one credit, inserts the comment, and records its Outbox event before success is returned. |
| Interaction analytics | Eventual consistency | An idempotent Consumer updates business aggregates independently of request handling. |
| Live-session interaction cutoff | Weak for Chat/Reaction; strict for Gift-effect comments | APIs reject after observing `ENDED`; acknowledged propagation-tail events remain durable, while the Gift transaction locks and checks the LiveSession. |
| Live-session and replay reads | Bounded stale reads | Redis accelerates reads; MySQL remains the source of truth. |
| Replay creation and media results | Idempotent monotonic transitions | One Replay references one ended session; duplicate provider events are harmless and `AVAILABLE` cannot be downgraded. |
| Replay comments | Read-after-write for the creator | The API returns only after the MySQL transaction commits. |

### Reliability targets

| Scenario | Expected behavior |
| --- | --- |
| Duplicate event delivery | Each durable Consumer produces one logical database effect. |
| Poison message | Retries are bounded and the message enters the correct DLQ. |
| RabbitMQ unavailable during Chat or Reaction | Chat receives no accepted ACK; a Reaction frame produces no published effect and is not retried by the client protocol. |
| Required Persistence binding missing | Topology audit alerts, messaging readiness fails, and Chat/Reaction send admission closes even if real-time queues still exist. |
| RabbitMQ unavailable after a gift-effect commit | The Outbox retains and retries the event within its bounded budget; exhaustion moves it to `FAILED` for alerting and controlled retry without losing committed business data. |
| Single RabbitMQ process or container restart | With retained broker storage, runtime processes reconnect and the privileged initializer/auditor restores and validates topology before send gates reopen. This does not cover broker-host or disk loss. |
| Redis limiter outage | For at most 30 seconds, local user buckets use 20% of normal capacity and each local room bucket receives one `MAX_API_REPLICAS` share of the 20% degraded cluster budget; the system then fails closed with `503`. |
| Redis cache outage | Reads use deadline- and semaphore-bounded MySQL fallback; saturation returns `503` rather than exhausting the database. |
| Redis outage during Gift use | Grant selection and credit consumption continue transactionally in MySQL without Redis correctness dependencies. |
| Persistence Consumer or MySQL outage | Chat events remain queued, retries are bounded, and pressure does not propagate without limit. |
| Slow Analytics Consumer | Analytics lag grows independently without blocking chat acceptance or persistence. |
| API crash after a business commit | The committed Outbox event remains publishable. |
| Concurrent Chat key with conflicting payloads | One payload wins at MySQL commit; the other is audited and acknowledged without overwrite or automatic redrive. |
| Session ends under interaction load | APIs converge through `live_session.ended`; acknowledged tail events remain stored, while new Gift-effect comments fail the transactional `LIVE` check. |
| `SIGTERM` during load | New work stops and in-flight work is drained or safely returned within the deadline. |
| WebSocket churn | No data race, panic, blocked registration, empty-room leak, or leaked connection. |

### Verification strategy

| Layer | Scope |
| --- | --- |
| Unit | State transitions, UUIDv4 message validation, session-scoped event-ID derivation, request-hash conflict classification, preserved event time, Reaction intent semantics, Grant eligibility and lock retry, Outbox fencing, replica-aware fallback-budget calculation, and retry classification. |
| Integration | Real MySQL, Redis, and RabbitMQ tests for conflicting `(live_session_id, user_id, message_id)` publication, unchanged `occurred_at` across Publisher retries and DLQ redrive, per-Consumer Inbox isolation, redrive-age rejection, confirms, mandatory returns, required-binding send-gate closure, isolated Persistence and Analytics retry/DLQ paths, Replay transition guards, Token Bucket behavior, and cursor-based eventual discovery. |
| Race and fuzz | Concurrent Hub/Room lifecycle, slow clients, malformed events, invalid envelopes, and bounded WebSocket input. |
| End to end | Two API instances verify conditional Chat acceptance, cross-instance broadcast, authoritative-history reconciliation, bounded reconnect attempts, weak session-end cutoff, strict Gift cutoff, reaction-by-type aggregation, Gift credits, and Insights results. |
| Load | Sustained chat and reaction bursts measure throughput, p95/p99 latency, confirm latency, queue depth, persistence delay, and slow-client removal. |
| Failure injection | Restart or pause API, Worker, RabbitMQ, Redis, and MySQL; remove a required binding; expire and reclaim an Outbox lease; verify Redis `HEALTHY → DEGRADED_LOCAL → FAIL_CLOSED → HEALTHY`, cache-fallback saturation, crash windows, duplicate delivery, retry/DLQ, and graceful `SIGTERM`. |

Tests run in a production-like environment, not claimed as real production traffic. Commands, expected behavior, measurements, recovery time, and residual risks are recorded in `PRODUCTION_READINESS.md`.

## 7. Implementation Roadmap

1. **Domain model:** introduce `LiveSession` with weak and strict cutoff paths, guarded Replay transitions, `ChatMessage`, `ReactionAggregate`, `GiftEffectGrant`, fenced Outbox leases, per-Consumer Inbox records, and interaction aggregate models; migrate the existing `Video` feed to Replay.
2. **Foundation:** add versioned migrations, configuration validation, secrets cleanup, typed errors, health endpoints, and a CI baseline.
3. **WebSocket safety:** authenticate connections; establish single room ownership, bounded queues, heartbeats, graceful shutdown, and race tests.
4. **Messaging foundation:** replace direct named queues with a versioned interaction exchange, stable event envelopes, separated topology/runtime permissions, required-binding audits, publisher confirms, mandatory routing, reconnection, one per-API real-time queue, isolated retry/DLQ paths, and fenced Outbox publication.
5. **Persistent chat:** add client-generated UUIDv4 message IDs, request hashes, authenticated user/message uniqueness, stable event IDs, API-assigned event time, direct confirmed publication, first-persisted conflict handling, exponential reconnect reconciliation, and client deduplication.
6. **Live reactions:** add deliberately non-retrying client intent, stable internal publication IDs, typed append-only reactions, real-time effects, per-type aggregation, and burst tests.
7. **Gift effects:** add hashed trusted gift results, database-time Grant eligibility, consistent LiveSession/Grant locking, bounded transaction retry, idempotent credit consumption, and Transactional Outbox publication.
8. **Analytics:** add an idempotent Consumer, per-session/per-minute business aggregates, and a Live Interaction Insights API; keep operational telemetry in Prometheus.
9. **Cache and degradation:** add distributed Redis Token Buckets, replica-aware local user/room fallback for at most 30 seconds, validated `MAX_API_REPLICAS`, fail-closed admission, request coalescing, semaphore-bounded MySQL fallback, circuit breaking, and cache invalidation.
10. **Verification:** automate the verification matrix and publish repeatable results in `PRODUCTION_READINESS.md`.

## 8. Out of Scope

- Media upload, transcoding, live video delivery, storage, or CDN integration
- Payment processing, refunds, or financial-grade ledger correctness
- Kafka migration, Kafka Streams, Kubernetes deployment, or GitOps
- Multi-node RabbitMQ clustering, Quorum Queue failover, or claims of broker-node high availability
- Full presence tracking or authoritative online-user state
- Strict global ordering across all live sessions
- A zero-tail, globally synchronized cutoff for Chat and Reaction at `ended_at`
- Permanently retaining every raw interaction event in the message broker
- Synchronized chat playback against the recorded-media timeline
- Automatic media-processing retry after a Replay enters `FAILED`
- Replacing the HTTP framework or ORM
- Claiming production readiness based only on local testing
