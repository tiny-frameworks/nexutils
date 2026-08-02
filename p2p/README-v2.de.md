
# nexutils/rpc
<sup>the *p2p module*, part of **GSF-nexutils**, member of the **tiny-frameworks** family</sup>

---

`nexutils/rpc` ist ein leichtgewichtiges, performantes **JSON-RPC 2.0 über WebSocket** Paket für Go. Es basiert auf [`github.com/coder/websocket`](https://www.google.com/search?q=https://codeberg.org/coder/websocket) mit automatischem Session-Management, Heartbeats und resilienter Client-Steuerung. Es ist speziell für den Einsatz in **Peer-to-Peer (P2P)**-Szenarien und verteilten Systemen konzipiert.

---

## Features

* **Symmetrische Nodes:** Jede Node kann gleichzeitig WebSocket-Server (Incoming RPCs) und Client (Outgoing RPCs) sein.
* **Typensicheres JSON-RPC 2.0:** Volle Unterstützung für synchrone Methodenaufrufe (`Call`) und asynchrone Einweg-Events (`Notify`).
* **Managed Client Architecture:** Entkoppelter, zustandsbasierter Auto-Reconnect mit konfigurierbarem Exponential Backoff & Jitter.
* **Automatische Re-Authentifizierung:** Nahtloser Fallback-Mechanismus von Session-Tokens auf Zugangsdaten bei Server-Neustarts.
* **Context-Driven:** Komplette Cancellation-Support und strikte Request-Timeouts gegen Goroutine-Leaks.

## Modul-Übersicht

| Komponente | Aufgabe |
| :--- | :--- |
| **`Node`** | Verwaltet den lokalen HTTP/WebSocket-Listener, Handler-Registrierungen und aktive Peer-Verbindungen. |
| **`Peer`** | Bipolare Verbindung zu einem Remote-Knoten. Zuständig für Frame-Handling, Pending-Request-Matching und Multiplexing. |
| **`ManagedClient`** | Auto-Reconnect Daemon mit integrierter State-Machine (`Connecting` -> `Authenticating` -> `Ready`). |

## Quickstart: Managed Client (Resilient)

Für langlebige Client-Verbindungen, die Netzausfälle und Neustarts des Gegenübers autonom überstehen müssen:

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

    // Resilienter Client mit Auto-Reconnect & Re-Auth
    client := node.ConnectWithAutoReconnect("ws://127.0.0.1:8080/ws", rpc.ReconnectConfig{
        InitialInterval: 1 * time.Second,
        MaxInterval:     10 * time.Second,
        MaxRetries:      10,
    })
    client.SetCredentials("georg", "secret")
    defer client.Close()

    // Synchroner Call mit explizitem Timeout
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    var response string
    if err := client.Call(ctx, "payment.process", "Order_123", &response); err != nil {
        // Fehlerbehandlung (z.B. "client connection is not ready")
        return
    }
}

```

## Direct Peer Connection (Statisch)

Für kontrollierte Ad-hoc-Verbindungen ohne Hintergrund-Management:

```go
peer, err := node.ConnectToPeer("ws://127.0.0.1:8080/ws")
if err != nil {
    // Fehler beim Verbindungsaufbau
}

ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

var result string
if err := peer.Call(ctx, "ping", nil, &result); err != nil {
    // Statischer Aufruf fehlgeschlagen
}

```

