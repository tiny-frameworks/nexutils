[![Codeberg Release](https://img.shields.io/codeberg/v/release/tiny-frameworks/nexutils?logo=codeberg&logoColor=white&color=2196F3)](https://codeberg.org/tiny-frameworks/nexutils)
[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)

## nexutils/p2p
<sup>the *p2p module*, part of **GSF-nexutils**, member of the **tiny-frameworks** family</sup>

---

`nexutils/p2p` is a lightweight, high-performance **JSON-RPC 2.0 over WebSocket** package for Go. It is built on top of [`github.com/coder/websocket`](https://codeberg.org/coder/websocket) with
automatic session management, heartbeats, and resilient client control. It is specifically designed
for **Peer-to-Peer (P2P)** scenarios and distributed systems.

---

## Key Features

* **Delegate-Driven Architecture (`NexDelegate`):** Decoupled, object-oriented lifecycle management and RPC dispatching replacing map-based route handlers.
* **True Peer Symmetry:** Every node (`Node`) can act simultaneously as a server (incoming connections) and as a client (outgoing connections).
* **Managed Client Architecture:** Decoupled, stateful auto-reconnect daemon featuring configurable Exponential Backoff & Jitter.
* **Type-Safe JSON-RPC 2.0:** Full support for synchronous method calls (`Call`) and asynchronous one-way events (`Notify`).
* **Context-Driven:** Full cancellation support and strict request timeouts to prevent goroutine leaks.
* **Heartbeat & Time Sync:** Integrated liveness check driven by the client role. The receiving peer automatically responds with `pong` and a precise UTC timestamp.
* **No TLS Overhead in Code:** Designed for secure internal environments or operation behind reverse proxies (e.g., **Caddy**), which handle TLS termination more efficiently.
* **Structured Errors:** Full integration with native JSON-RPC error codes and custom business error types.

---

## Architecture & Concepts

### 1. Components

The package is split into the following core responsibilities:

| Component / File | Responsibility |
| --- | --- |
| **`node.go` (`Node`)** | Primary lifecycle manager. Starts/stops the HTTP/WebSocket listener, unpacks `Options` to internal fields, and delegates inbound messages/lifecycle events. |
| **`peer.go` (`Peer`)** | Bipolar WebSocket connection to a remote node. Handles frame processing, pending request matching, authentication state, and heartbeats. |
| **`delegate.go`** | Defines `NexDelegate` interface, `UserAuthenticator` interface, and `DefaultNexDelegate` base struct for handling peer validation, requests, notifications, and disconnects. Manages built-in `"auth"` requests and in-memory token `SessionStore` for reconnects. |
| **`client.go` (`ManagedClient`)** | Resilient auto-reconnect daemon running a state machine (`Connecting` -> `Authenticating` -> `Ready`) for long-lived client connections. |
| **`protocol.go`** | JSON-RPC 2.0 specification (requests, responses, errors) and standard error definitions. |
| **`options.go`** | Configuration parameters for timeouts, ports, reconnect policies, and delegate injection. |

### 2. Peer-to-Peer (P2P) Principle & Delegate Dispatching

Once the initial WebSocket handshake via HTTP GET is completed, the connection upgrades to the bidirectional WebSocket protocol. Inbound requests and lifecycle events bypass message queues and route maps, executing directly via the injected `NexDelegate`.

```
│   [ Node A ]                                  │   [ Node B ]

```

```

(Port :8080 / Server)                           (Port :8081 / Server)
▲                                               │
│────── ConnectToPeer("ws://nodeA/ws") ─────────│  (RoleOutbound)
│  or ConnectWithAutoReconnect(...)             │
│                                               ▼
├─────────────── Request: "auth" ───────────────┤  ──> NexDelegate.OnRequest()
├─────────────── Request: "heartbeat" ──────────┤  ──> NexDelegate.OnRequest()
├─────────────── Notify: "event" ───────────────┤  ──> NexDelegate.OnNotification()

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

### 1. Custom Delegate Implementation

Embed `rpc.DefaultNexDelegate` and override only the lifecycle or request hooks you need:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

type MyDelegate struct {
    rpc.DefaultNexDelegate
}

// Peer connection validation (Auth / Filtering)
func (d *MyDelegate) ValidatePeer(peer *rpc.Peer) bool {
    log.Printf("Validating peer connection from %s", peer.RemoteAddr())
    return true
}

func (d *MyDelegate) OnPeerConnected(peer *rpc.Peer) {
    log.Printf("Peer connected: %s", peer.ID)
}

func (d *MyDelegate) OnPeerDisconnected(peer *rpc.Peer, err error) {
    log.Printf("Peer disconnected: %s (Reason: %v)", peer.ID, err)
}

// Request & RPC Dispatching
func (d *MyDelegate) OnRequest(ctx context.Context, peer *rpc.Peer, method string, params []byte) (any, error) {
    switch method {
    case "ping":
        return "pong", nil
    case "add":
        return map[string]int{"result": 42}, nil
    default:
        // Returns standard MethodNotFound (-32601) JSON-RPC error
        return d.DefaultNexDelegate.OnRequest(ctx, peer, method, params)
    }
}

// One-way notification handler
func (d *MyDelegate) OnNotification(ctx context.Context, peer *rpc.Peer, method string, params []byte) {
    log.Printf("Notification received [%s]: %s", method, string(params))
}

```

---

### 2. Authentication & Session Management

`DefaultNexDelegate` provides built-in handling for the `"auth"` RPC method using token-based sessions:

1. **Injecting an Authenticator**: Inject a custom `UserAuthenticator` into your delegate or `Node`. If omitted, a `DummyAuthenticator` (`georg`/`secret`) is used as a fallback.
   ```go
   myDelegate := &MyDelegate{}
   myDelegate.SetAuthenticator(myUserStore) // Implements rpc.UserAuthenticator


2. **Protecting RPC Methods**: Secure your business endpoints using `peer.IsAuthorized()` and retrieve identity via `peer.Username()`:

```go
func (d *MyDelegate) OnRequest(ctx context.Context, peer *rpc.Peer, method string, params []byte) (any, error) {
    switch method {
    case "protected.action":
        if !peer.IsAuthorized() {
            return nil, &rpc.JsonRPCerror{Code: rpc.UnAuthorized, Message: rpc.StdError[rpc.UnAuthorized]}
        }
        log.Printf("Action executed by %s", peer.Username())
        return "ok", nil
    default:
        return d.DefaultNexDelegate.OnRequest(ctx, peer, method, params)
    }
}
```


---

### 3. Minimal Server (Node)

Initialize the server using `rpc.Options`. The options are automatically unpacked into internal `Node` fields upon creation.

```go
package main

import (
    "log"
    "time"

    "codeberg.org/tiny-frameworks/nexutils/logger"
    "codeberg.org/tiny-frameworks/nexutils/p2p/rpc"
)

func main() {
    _ = logger.SetupLogging(nil)

    // Create node with custom delegate
    node := rpc.NewNode(rpc.Options{
        Addr:              ":8080",
        HeartbeatInterval: 10 * time.Second,
        Delegate:          &MyDelegate{},
    })

    // Start server (blocking)
    if err := node.Start(); err != nil {
        log.Fatalf("Node crashed: %v", err)
    }
}

```

### 4. Executable Example

A a complete, runnable end-to-end example (`main.go`) demonstrating two nodes (`Node A` as Server
and `Node B` as Client) interacting via custom `NexDelegate` implementations:

```go
go run main.go
```

---

### 5. Managed Client with Auto-Reconnect (Resilient)

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
        Delegate:          &MyDelegate{},
    })
    go node.Start()
    defer node.Stop()

    // Resilient client with auto-reconnect
    client := node.ConnectWithAutoReconnect("ws://127.0.0.1:8080/ws", rpc.ReconnectConfig{
        InitialInterval: 1 * time.Second,
        MaxInterval:     10 * time.Second,
        MaxRetries:      10,
    })
    defer client.Close()

    //Set credentials for automated auth & token-based session restoration on reconnects
    client.SetCredentials("admin", "secret")

    // Synchronous call with explicit timeout
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    var response string
    if err := client.Call(ctx, "ping", nil, &response); err != nil {
        // Error handling
        return
    }
}

```

---

### 6. Direct Peer Connection (Static)

For controlled ad-hoc connections without background management:

```go
peer, err := node.ConnectToPeer("ws://127.0.0.1:8080/ws")
if err != nil {
    // Connection establishment error
}

ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

var result string
if err := peer.Call(ctx, "add", nil, &result); err != nil {
    // Static RPC call failed
}

```

---

## Configuration Options (`Options`)

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `Addr` | `string` | `":8080"` | Listen address for the WebSocket HTTP server. |
| `HeartbeatInterval` | `time.Duration` | `15s` | Interval for client-side automated ping/heartbeat checks. |
| `ShutdownDelay` | `time.Duration` | `5s` | Graceful shutdown timeout for the HTTP server. |
| `WriteReadLimit` | `int64` | `1048576` (1MB) | Maximum allowed frame size in bytes. |
| `Delegate` | `NexDelegate` | `&DefaultNexDelegate{}` | Delegate interface handling node events and requests. |

---

## Running Tests

An integration test suite is included in the package directory:

```bash
go test -v ./...

```

---

## Organizational & Standards

* **Copyright:** © 2026 Georg Hagn.
* **Namespace:** `codeberg.org/tiny-frameworks/nexutils/p2p/rpc`
* **License:** Apache License, Version 2.0.

*GSF-nexutils/p2p is an independent open-source project and is not affiliated with any corporation of a similar name.*

---

## Contact

If you have questions or feedback, feel free to reach out:

📧 *georghagn [at] tiny-frameworks.io*

```

```
