
## Microservice Communication & P2P RPC Demo Suite for nexutils/p2p

This directory contains example microservices demonstrating the architectural patterns, symmetry, and
resilience capabilities provided by the `nexutils/p2p/rpc` package (part
of the **GSF-nexutils** ).

---

### Architecture & Service Roles

```
               ┌─────────────────────────┐
               │     payment-service     │
               │   (WebSocket Server)    │
               └────────────▲────────────┘
                            │
            ┌───────────────┴─────────────────┐
            │                                 │
 [Static Direct Connection]            [   Managed Client    ]
      Ad-hoc Call                      Auto-Reconnect & Re-Auth
            │                                     │
 ┌──────────┴──────────┐               ┌──────────┴──────────┐
 │    order-service    │               │   secret-service    │
 └─────────────────────┘               └─────────────────────┘

```

#### 1. `payment-service` (The Central Kernel)
* **Role:** Central payment authority and connection hub.
* **Behavior:** Verifies client credentials, issues session tokens, handles payment requests (`payment.process`), and periodically sends system broadcasts to all connected peers.

#### 2. `secret-service` (Decoupled, High-Availability Worker)
* **Role:** Maintains a persistent, autonomous worker connection to the `payment-service`.
* **Architecture:** Uses `ConnectWithAutoReconnect` & `ManagedClient`.
* **Resilience:** 
  * Survives remote server crashes and restarts without terminating its own runtime.
  * Automatically retries connection using session tokens.
  * Seamlessly falls back to username/password credentials when session tokens expire or reset, obtaining a new token and resuming operation autonomously.

#### 3. `order-service` (Compact Direct Client)
* **Role:** Executes focused, ad-hoc transactions.
* **Architecture:** Uses `ConnectToPeer` (static direct peer connection).
* **Resilience:** Fails immediately if the target peer is unreachable. Allows the calling goroutine to terminate cleanly or trigger application-level graceful shutdown via context cancellation.

---

### The nexutils-p2p Role Paradigm: From Connection to Symmetry

The core philosophy behind these examples is **decoupling physical connection establishment from communication roles**.

#### 1. Initial Connection Setup (Asymmetric)
At startup, services use traditional client/server semantics solely to establish physical WebSocket sockets:
* **Payment Service:** Serves as the initial listener (waiting for incoming TCP/WS connections).
* **Order / Secret Services:** Serve as initial clients (initiating outbound HTTP/WS requests).

#### 2. Established Communication (Symmetrical)
Once the WebSocket handshake completes, any traditional client/server distinction disappears entirely:
* **Unified Nodes:** All connected peers operate as equal `nexIOnode` / `Peer` instances.
* **Full-Duplex RPC:** The `payment-service` can invoke RPC methods on the `order-service` (e.g., `UpdateOrderStatus`) just as easily as the `order-service` invokes `payment-service` procedures (e.g., `ProcessPayment`).
* **Shared Core Logic:** All instances share identical listen loops, handler registration mechanisms, and resilience roadmaps.

---

### Startup Configuration Matrix

| Feature | Initial Server (e.g., Payment) | Initial Client (e.g., Order / Secret) |
| --- | --- | --- |
| **Provider Setup** | `provider.Listen(ctx, addr, foundChan)` | `provider.Dial(ctx, addr)` |
| **Node Creation** | `NewNode(connFromChan, provider, "")` | `NewNode(dialedConn, provider, addr)` |
| **Start Command** | `go node.Listen(ctx)` | `go node.Listen(ctx)` |
| **Subsequent Role** | **Peer** (Symmetric P2P) | **Peer** (Symmetric P2P) |
| **Capabilities** | `Call`, `Notify`, `RegisterHandler` | `Call`, `Notify`, `RegisterHandler` |

---

### Running Demo Scenarios

Open three separate terminal windows side-by-side to observe system logs, identity tracking, and connection heartbeats:

```bash
# Terminal 1: Start Central Payment Service (Listening on :8080)
go run cmd/services/payment-service/main.go

# Terminal 2: Start Resilient Managed Client Worker
go run cmd/services/secret-service/main.go

# Terminal 3: Start Ad-hoc Direct Client
go run cmd/services/order-service/main.go

```

---

### Resilience & Auto-Reconnect Test

Follow these steps to demonstrate automatic re-authentication and exponential backoff in action:

1. Start `payment-service` and `secret-service` in separate terminals.
2. **Kill the Payment Service:** Press `Ctrl + C` in Terminal 1.
3. **Observe Worker Behavior:**
* The `secret-service` transitions into the `reconnecting...` state.
* In-flight ticker requests are safely intercepted without process crashes.
* The reconnect loop engages exponential backoff with jitter (1s, 2s, 4s...).

4. **Restart Payment Service:** Run the `payment-service` command again in Terminal 1.
5. **Observe Automated Recovery:**
* Client reconnects to the newly started TCP socket.
* Initial token auth fails because the server lost in-memory sessions during reboot.
* Automatic fallback uses fallback credentials (username/password) to authenticate.
* Server issues a new token, client state returns to `Ready`.
* Transactions resume seamlessly!

---

### Key Observations for Inspection

* **Log Identity:** Thanks to logger contextual adapters (`With()`), every log entry explicitly identifies which microservice instance is issuing the output.
* **Parallel Processing:** The central payment server spawns separate goroutines for each incoming client, processing concurrent calls smoothly.
* **Ad-hoc Failure vs. Managed Resilience:** Stopping the payment server causes `order-service` (direct peer) to exit quickly while `secret-service` (managed client) waits patiently and reconnects.



