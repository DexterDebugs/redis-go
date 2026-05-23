# redis-go

A Redis-compatible in-memory data store, built from scratch in Go.

<!-- TODO: add a demo GIF here showing redis-cli connecting and a TTL countdown -->

redis-go is a small Redis server written from the ground up — no Redis libraries, no frameworks. It speaks the real Redis wire protocol (RESP), so the official `redis-cli` connects to it and works as if it were talking to real Redis. It handles multiple clients concurrently and supports core key-value operations including time-based key expiration.

## Why I built it

I built redis-go to understand what actually happens beneath the backend tools I use every day. Like most developers, I'd used Redis without knowing how it works. So I rebuilt a small version from scratch in Go — and in the process learned how to handle raw TCP connections, parse a real wire protocol (RESP), keep an in-memory store correct under concurrent access with mutexes, and design key expiration as a deliberate tradeoff (lazy deletion over a background sweeper). The goal was never to replace Redis — it was to stop treating it as a black box. Every design decision in this project is one I can explain and defend.

## Features

| Command | Description | Example |
|---|---|---|
| `PING` | Health check; replies `PONG` | `PING` → `PONG` |
| `SET` | Store a key-value pair | `SET name Alex` → `OK` |
| `GET` | Retrieve a value by key | `GET name` → `"Alex"` |
| `DEL` | Delete a key; returns count deleted | `DEL name` → `(integer) 1` |
| `EXISTS` | Check if a key exists | `EXISTS name` → `(integer) 1` |
| `EXPIRE` | Set a key to expire after N seconds | `EXPIRE name 60` → `(integer) 1` |
| `TTL` | Seconds until a key expires | `TTL name` → `(integer) 60` |

- Speaks the real RESP protocol — compatible with the official `redis-cli`
- Handles multiple concurrent clients via goroutines
- Thread-safe in-memory store guarded by a mutex
- Lazy key expiration (expired keys are reclaimed on access)
- Robust against malformed input — bad commands return errors instead of crashing the server

## How it works

A command travels through the server in five stages:

1. **TCP** — the client connects over TCP on port 6379 (Redis's default port).
2. **Parse** — the RESP parser reads the raw bytes and turns them into a Go slice, e.g. `["SET", "name", "Alex"]`.
3. **Dispatch** — a switch statement routes the command to its handler.
4. **Execute** — the handler reads or modifies the in-memory store (under a mutex lock).
5. **Reply** — the handler writes a RESP-formatted response back over the connection.

The TCP and parsing layers are identical to what any networked server needs; the dispatch and execution layers are what make it specifically a Redis server. Adding a new command means adding one case to the switch.

<!-- TODO: add architecture diagram image here -->

## Running it

Requires Go 1.18+ installed.

```bash
# Clone the repo
git clone https://github.com/DexterDebugs/redis-go.git
cd redis-go

# Run the server (listens on port 6379)
go run main.go
```

In a second terminal, connect with the real Redis CLI:

```bash
redis-cli

127.0.0.1:6379> set session abc
OK
127.0.0.1:6379> get session
"abc"
127.0.0.1:6379> expire session 10
(integer) 1
127.0.0.1:6379> ttl session
(integer) 9
127.0.0.1:6379> get session
(nil)          # after 10 seconds, the key has expired
```

## Design decisions

**Lazy expiration over a background sweeper.** Expired keys are not proactively deleted on a timer. Instead, every access (`GET`, `EXISTS`, `TTL`) first checks whether the key has expired and deletes it if so. This needs no extra goroutines or timing loops, and it's a legitimate real-world strategy — it's half of what real Redis does. The tradeoff: an expired key that's never accessed again sits in memory until something touches it. Real Redis combines lazy expiration with active random sampling.

**A mutex for concurrency.** Each client runs in its own goroutine, and Go maps are not safe for concurrent access — concurrent writes cause a hard crash. A single mutex guards every read and write to the store. Critical sections are kept minimal: only the in-memory map work happens under the lock; network I/O happens after the lock is released, so a slow client can't block other clients.

**Direct RESP parsing, no dependencies.** The wire protocol is parsed by hand rather than using a library, to actually understand how Redis clients and servers communicate at the byte level.

## What I learned

- **Instrument before assuming there's a bug.** My EXPIRE/TTL looked broken — I set a 10-second expiry but TTL reported 3, then the key vanished. Adding a debug print proved the expiry was stored correctly; the "3" was just real elapsed time while I typed the command. The code was right; my mental model of timing was wrong. Printing the actual values resolves a "mysterious bug" faster than reasoning in the dark.

- **RESP prefix bytes are a contract.** Each reply type has a leading byte — `*` for arrays, `$` for bulk strings, `:` for integers, `+` for simple strings, `-` for errors. I initially used `$` for count replies, which told the client "a bulk string is coming"; it then waited for a payload that never arrived and hung. The prefix byte tells the client how to interpret what follows — mismatch it and you silently break the protocol.

- **Decide deliberately whether to capture or ignore each return value.** Go forces you to acknowledge what a function returns. I learned to capture the error when ignoring it risks a crash (my index-out-of-range panics came from not stopping after a validation failure), and to ignore values deliberately with `_` only when I genuinely don't need them. That discipline prevents whole classes of bugs.

- **Lock placement defines correctness in concurrent code.** Go's `sync.Mutex` isn't reentrant, so my expiry-check helper does no locking of its own — the caller must hold the lock, or it deadlocks. Network I/O never happens while holding the lock, because a slow client would block every other goroutine. And the position of `Lock`/`Unlock` defines the critical section: too wide stalls everything, too narrow causes races.

## What's next (v2)

- A background sweeper goroutine to reclaim expired keys that are never accessed again
- More data types: lists (`LPUSH`/`RPOP`) to support job queues, hashes for structured objects
- Persistence via an append-only log, so data survives a restart
- Replacing the two parallel maps (value + expiry) with a single struct to prevent them drifting out of sync
- Benchmarks measuring throughput and behavior under concurrent load

## Tech

Go 1.18+ · standard library only (`net`, `bufio`, `sync`, `time`, `strconv`) · RESP protocol · zero external dependencies
