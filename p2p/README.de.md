
# nexutils/rpc

the *p2p module*, part of **GSF-nexutils**, member of the **tiny-frameworks** family

---

`nexutils/rpc` ist ein leichtgewichtiges, performantes **JSON-RPC 2.0 über WebSocket** Paket für Go. Es basiert auf [`github.com/coder/websocket`](https://www.google.com/search?q=https://codeberg.org/coder/websocket) mit automatischem Session-Management, Heartbeats und resilienter Client-Steuerung. Es ist speziell für den Einsatz in **Peer-to-Peer (P2P)**-Szenarien und verteilten Systemen konzipiert.

---

## Key Features

* **Echte Peer-Symmetrie:** Jeder Knoten (`Node`) kann zeitgleich als Server (eingehende Verbindungen) und als Client (ausgehende Verbindungen) agieren.
* **Managed Client Architecture:** Entkoppelter, zustandsbasierter Auto-Reconnect mit konfigurierbarem Exponential Backoff & Jitter.
* **Automatische Re-Authentifizierung:** Nahtloser Fallback-Mechanismus von Session-Tokens auf Zugangsdaten bei Server-Neustarts.
* **Typensicheres JSON-RPC 2.0:** Volle Unterstützung für synchrone Methodenaufrufe (`Call`) und asynchrone Einweg-Events (`Notify`).
* **Context-Driven:** Komplette Cancellation-Support und strikte Request-Timeouts gegen Goroutine-Leaks.
* **Heartbeat & Time-Sync:** Integrierter Liveness-Check von der Client-Rolle aus. Der Empfänger antwortet automatisch mit `pong` und einem präzisen UTC-Timestamp.
* **Kein TLS-Ballast im Code:** Entwickelt für gesicherte Umgebungen oder den Betrieb hinter Proxies (z. B. **Caddy**), die TLS-Terminierung deutlich effizienter handhaben.
* **Entkoppelte User-Verwaltung:** Über das `UserAuthenticator`-Interface kann jede beliebige Datenbank, LDAP oder Custom-Auth-Logik angedockt werden.
* **Strukturierte Fehler:** Vollständige Integration mit `nexutils/errors` (Fehler-Details werden im `data`-Feld von JSON-RPC übertragen).

---

## Architektur & Konzepte

### 1. Die Komponenten

Das Paket teilt sich in folgende Kernzuständigkeiten auf:

| Komponente / Datei | Aufgabe |
| --- | --- |
| **`node.go` (`Node`)** | Haupt-Lifecycle-Manager. Startet/stoppt den HTTP/WebSocket-Listener, verwaltet Handler-Registrierungen und stellt ausgehende Verbindungen her. |
| **`peer.go` (`Peer`)** | Bipolare WebSocket-Verbindung zu einem Remote-Knoten. Steuert Read/Write-Loops, Frame-Handling, Pending-Request-Matching, Auth-State und Heartbeats. |
| **`client.go` (`ManagedClient`)** | Resilienter Auto-Reconnect Daemon mit integrierter State-Machine (`Connecting` -> `Authenticating` -> `Ready`) für dauerhafte Client-Verbindungen. |
| **`router.go`** | Registriert RPC-Methoden (`RegisterHandler`), verwaltet Session-Tokens und verteilt eingehende Anfragen (`dispatchLoop`).|
| **`protocol.go`** | JSON-RPC 2.0 Spezifikation (Requests, Responses, Errors) und Mapper für `nexutils/errors`. |
| **`options.go`** | Konfiguration für Timeouts, Ports, Reconnects und Heartbeat-Intervalle. |

### 2. Peer-to-Peer (P2P) Prinzip

Nachdem der initiale WebSocket-Handshake via HTTP GET vollzogen ist, schaltet die Verbindung auf das bidirektionale WebSocket-Protokoll um. Ab diesem Moment gibt es **keine feste Server/Client-Hierarchie mehr**: Beide Seiten können völlig asynchron JSON-RPC Requests senden und empfangen.

```
       [ Node A ]                                      [ Node B ]
(Port :8080 / Server)                           (Port :8081 / Server)
          ▲                                               │
          │────── ConnectToPeer("ws://nodeA/ws") ─────────│  (RoleOutbound)
          │  oder ConnectWithAutoReconnect(...)           │
          │                                               ▼
          ├─────────────── Request: "auth" ───────────────┤
          ├─────────────── Request: "heartbeat" ──────────┤ (Ticker)
          ├─────────────── Request: "customMethod" ───────┤

```

---

## Installation & Import

```bash
go get codeberg.org/tiny-frameworks/nexutils/rpc

```

```go
import "codeberg.org/tiny-frameworks/nexutils/rpc"

```

---

## Schnellstart & Beispiele

### 1. Minimaler Server (Node)

```go
package main

import (
	"log"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/logger"
	"codeberg.org/tiny-frameworks/nexutils/rpc"
)

func main() {
	// Logger initialisieren
	_ = logger.SetupLogging(nil)

	// Node erstellen
	node := rpc.NewNode(rpc.Options{
		Addr:              ":8080",
		HeartbeatInterval: 10 * time.Second,
	})

	// Eigene RPC-Methode registrieren
	node.RegisterHandler("add", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
		// Logik...
		return map[string]int{"result": 42}, nil
	})

	// Server starten (blockiert)
	if err := node.Start(); err != nil {
		log.Fatalf("Node abgestürzt: %v", err)
	}
}

```

---

### 2. Managed Client mit Auto-Reconnect (Resilient)

Für langlebige Client-Verbindungen, die Netzausfälle und Neustarts des Gegenübers autonom überstehen müssen:

```go
package main

import (
	"context"
	"time"

	"codeberg.org/tiny-frameworks/nexutils/rpc"
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

---

### 3. Direct Peer Connection (Statisch)

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

---

### 4. Eigene User-Verwaltung einbinden (`UserAuthenticator`)

Standardmäßig nutzt das System einen Dummy-Check (`georg` / `secret`). Du kannst jederzeit deine eigene Datenbank-Logik andocken:

```go
type MyDatabaseAuth struct {
	// z. B. db *sql.DB
}

// Implementiert rpc.UserAuthenticator
func (a *MyDatabaseAuth) Authenticate(ctx context.Context, username, password string) bool {
	// Hier eigene DB-/Hash-Prüfung durchführen:
	return username == "admin" && password == "super-secret"
}

func main() {
	node := rpc.NewNode(rpc.Options{Addr: ":8080"})

	// Eigene Auth-Logik injecten
	node.SetAuthenticator(&MyDatabaseAuth{})

	node.Start()
}

```

---

### 5. Geschützte Methoden schreiben

Im Handler kann einfach geprüft werden, ob der anfragende Peer authentifiziert ist:

```go
node.RegisterHandler("getSecretData", func(p *rpc.Peer, req rpc.JsonRPCrequest) (any, *rpc.JsonRPCerror) {
	if !p.IsAuthorized() {
		return nil, &rpc.JsonRPCerror{
			Code:    rpc.NotAuthorized,
			Message: rpc.StdError[rpc.NotAuthorized],
		}
	}

	return "Streng geheime Daten", nil
})

```

---

## Auth & Reconnect Flow

Das Protokoll unterstützt Token-basierte Sessions. Dadurch müssen bei einem Verbindungsabbruch keine Passwörter erneut übertragen werden:

### 1. Erstmaliges Login (Username/Password)

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

### 2. Reconnect nach Verbindungsabbruch (Token / Auto-Re-Auth)

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

*(Sollte der Server neu gestartet sein und den Token nicht mehr kennen, führt der `ManagedClient` im Hintergrund automatisch einen Fallback auf die hinterlegten Zugangsdaten durch).*

---

## Heartbeat & Time-Sync

Der Node in der **Client-Rolle** (Outbound Peer) sendet automatisch im konfigurierten `HeartbeatInterval` ein Signal:

* **Outbound Peer sendet:** `{"jsonrpc": "2.0", "method": "heartbeat", "id": 1690000000}`
* **Inbound Peer antwortet:**

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

Dies dient zeitgleich als **Liveness-Check** und erlaubt es dem Client, seine Uhrzeit mit dem Server zu synchronisieren (spart eine extra `getTime`-Methode).

---

## Tests ausführen

Im Paketverzeichnis steht ein vollständiger Integrationstest bereit:

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

