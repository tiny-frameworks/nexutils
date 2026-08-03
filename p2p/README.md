
## nexutils/p2p
<sup>the *p2p module*, part of **GSF-nexutils**, member of the **tiny-frameworks** family</sup>

---

`nexutils/p2p` is a lightweight, high-performance **JSON-RPC 2.0 over WebSocket** package for Go. It is built on top
of [`github.com/coder/websocket`](https://www.google.com/search?q=https://codeberg.org/coder/websocket) with
automatic session management, heartbeats, and resilient client control It is specifically designed
for **Peer-to-Peer (P2P)** scenarios and distributed systems.

---

## Key Features

* **True Peer Symmetry:** Every node (`Node`) can act simultaneously as a server (incoming connections) and as a client (outgoing connections).
* **Managed Client Architecture:** Decoupled, stateful auto-reconnect daemon featuring configurable Exponential Backoff & Jitter.
* **Automatic Re-Authentication:** Seamless fallback mechanism from session tokens to credentials upon server restarts.
* **Type-Safe JSON-RPC 2.0:** Full support for synchronous method calls (`Call`) and asynchronous one-way events (`Notify`).
* **Context-Driven:** Full cancellation support and strict request timeouts to prevent goroutine leaks.
* **Heartbeat & Time Sync:** Integrated liveness check driven by the client role. The receiving peer automatically responds with `pong` and a precise UTC timestamp.
* **No TLS Overhead in Code:** Designed for secure internal environments or operation behind reverse proxies (e.g., **Caddy**), which handle TLS termination more efficiently.
* **Decoupled User Authentication:** Pluggable `UserAuthenticator` interface allows integration of any database, LDAP, or custom auth logic.
* **Structured Errors:** Full integration with `nexutils/errors` (error details are transmitted inside the JSON-RPC `data` field).

---

## Architecture & Concepts

### 1. Components

The package is split into the following core responsibilities:

| Component / File | Responsibility |
| --- | --- |
| **`node.go` (`Node`)** | Primary lifecycle manager. Starts/stops the HTTP/WebSocket listener, manages handler registrations, and establishes outgoing connections. |
| **`peer.go` (`Peer`)** | Bipolar WebSocket connection to a remote node. Handles frame processing, pending request matching, authentication state, and heartbeats. |
| **`client.go` (`ManagedClient`)** | Resilient auto-reconnect daemon running a state machine (`Connecting` -> `Authenticating` -> `Ready`) for long-lived client connections. |
| **`router.go`** | Registers RPC methods (`RegisterHandler`), manages session tokens, and dispatches incoming requests (`dispatchLoop`). |
| **`protocol.go`** | JSON-RPC 2.0 specification (requests, responses, errors) and mapping logic for `nexutils/errors`. |
| **`options.go`** | Configuration parameters for timeouts, ports, reconnect policies, and heartbeat intervals. |

### 2. Peer-to-Peer (P2P) Principle

Once the initial WebSocket handshake via HTTP GET is completed, the connection upgrades to the bidirectional WebSocket protocol. From this point forward, there is **no fixed server/client hierarchy**: both peers can asynchronously send and receive JSON-RPC requests.

```
       [ Node A ]                                      [ Node B ]
(Port :8080 / Server)                           (Port :8081 / Server)
          ▲                                               │
          │────── ConnectToPeer("ws://nodeA/ws") ─────────│  (RoleOutbound)
          │  or ConnectWithAutoReconnect(...)             │
          │                                               ▼
          ├─────────────── Request: "auth" ───────────────┤
          ├─────────────── Request: "heartbeat" ──────────┤ (Ticker)
          ├─────────────── Request: "customMethod" ───────┤

```

---

## Installation & Import

```bash
go get codeberg.org/tiny-frameworks/nexutils/p2p

```

```go
import "codeberg.org/tiny-frameworks/nexutils/p2p/rpc"

```

---

## Quickstart & Examples

### 1. Minimal Server (Node)

```go
package main

import (
	"log"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/logger"
	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

func main() {
	// Initialize logger
	_ = logger.SetupLogging(nil)

	// Create node
	node := rpc.NewNode(rpc.Options{
		Addr:              ":8080",
		HeartbeatInterval: 10 * time.Second,
	})

	// Register custom RPC handler
	node.RegisterHandler("add", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		// Logic...
		return map[string]int{"result": 42}, nil
	})

	// Start server (blocking)
	if err := node.Start(); err != nil {
		log.Fatalf("Node crashed: %v", err)
	}
}

```

---

### 2. Managed Client with Auto-Reconnect (Resilient)

For long-lived client connections that must autonomously survive network outages and remote server restarts:

```go
package main

import (
	"context"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

func main() {
	node := rpc.NewNode(rpc.Options{
		HeartbeatInterval: 5 * time.Second,
	})
	go node.Start()
	defer node.Stop()

	// Resilient client with auto-reconnect & re-auth
	client := node.ConnectWithAutoReconnect("ws://127.0.0.1:8080/ws", rpc.ReconnectConfig{
		InitialInterval: 1 * time.Second,
		MaxInterval:     10 * time.Second,
		MaxRetries:      10,
	})
	client.SetCredentials("georg", "secret")
	defer client.Close()

	// Synchronous call with an explicit timeout
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var response string
	if err := client.Call(ctx, "payment.process", "Order_123", &response); err != nil {
		// Error handling (e.g., "client connection is not ready")
		return
	}
}

```

---

### 3. Direct Peer Connection (Static)

For controlled ad-hoc connections without background management:

```go
peer, err := node.ConnectToPeer("ws://127.0.0.1:8080/ws")
if err != nil {
	// Connection establishment error
}

ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

var result string
if err := peer.Call(ctx, "ping", nil, &result); err != nil {
	// Static RPC call failed
}

```

---

### 4. Custom User Authentication (`UserAuthenticator`)

By default, the system includes a dummy check (`georg` / `secret`). You can attach your custom database or auth logic at any time:

```go
type MyDatabaseAuth struct {
	// e.g., db *sql.DB
}

// Implements rpc.UserAuthenticator
func (a *MyDatabaseAuth) Authenticate(ctx context.Context, username, password string) bool {
	// Perform custom DB/hash validation:
	return username == "admin" && password == "super-secret"
}

func main() {
	node := rpc.NewNode(rpc.Options{Addr: ":8080"})

	// Inject custom authenticator
	node.SetAuthenticator(&MyDatabaseAuth{})

	node.Start()
}

```

---

### 5. Protected RPC Handlers

Inside a handler, simply verify whether the requesting peer is authorized:

```go
node.RegisterHandler("getSecretData", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
	if !p.IsAuthorized() {
		return nil, &rpc.JsonRPCerror{
			Code:    rpc.NotAuthorized,
			Message: rpc.StdError[rpc.NotAuthorized],
		}
	}

	return "Top secret data", nil
})

```

---

## Auth & Reconnect Flow

The protocol supports token-based sessions. This avoids sending passwords repeatedly over the wire during reconnects:

### 1. Initial Login (Username/Password)

* **Request:**

```json
{
  "jsonrpc": "2.0",
  "method": "auth",
  "params": {"username": "georg", "password": "secret"},
  "id": 1
}

```

* **Response:**

```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "authenticated",
    "token": "4f8a1c9e2b...",
    "username": "georg"
  },
  "id": 1
}

```

### 2. Reconnect After Connection Loss (Token / Auto Re-Auth)

* **Request:**

```json
{
  "jsonrpc": "2.0",
  "method": "auth",
  "params": {"token": "4f8a1c9e2b..."},
  "id": 2
}

```

* **Response:**

```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "session restored",
    "token": "4f8a1c9e2b...",
    "username": "georg"
  },
  "id": 2
}

```

*(If the remote server restarted and no longer recognizes the session token, the `ManagedClient` automatically falls back to re-authenticating using stored credentials in the background).

---

## Heartbeat & Time Sync

The node in the **Client role** (Outbound Peer) automatically transmits a ping at the configured `HeartbeatInterval`:

* **Outbound Peer sends:** `{"jsonrpc": "2.0", "method": "heartbeat", "id": 1690000000}`
* **Inbound Peer responds:**

```json
{
  "jsonrpc": "2.0",
  "result": {
    "status": "pong",
    "time": "2026-07-30T19:18:31.123456789Z"
  },
  "id": 1690000000
}

```

This acts simultaneously as a **liveness check** and enables the client to synchronize its local clock with the server (eliminating the need for a separate `getTime` call).

---

## Running Tests

An integration test suite is included in the package directory:

```bash
go test -v ./...

```

---

## Organizational & Standards

* **Copyright:** © 2026 Georg Hagn.
* **Namespace:** `codeberg.org/tiny-frameworks/nexutils/rpc`
* **License:** Apache License, Version 2.0.

*GSF-nexutils/rpc is an independent open-source project and is not affiliated with any corporation of a similar name.*

---

## Contact

If you have questions or feedback, feel free to reach out:

📧 *georghagn [at] tiny-frameworks.io*

