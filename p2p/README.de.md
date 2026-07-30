
# nexutils/rpc
<sup>the *p2p module*, part of **GSF-nexutils**, member of the **tiny-frameworks** family</sup>

---

`nexutils/rpc` ist ein leichtgewichtiges, performantes **JSON-RPC 2.0 über WebSocket** Paket für Go. Es basiert auf [`github.com/coder/websocket`](https://www.google.com/search?q=https://codeberg.org/coder/websocket) und ist speziell für den Einsatz in **Peer-to-Peer (P2P)**-Szenarien und verteilten Systemen konzipiert.

---

## Key Features

* **Echte Peer-Symmetrie:** Jeder Knoten (`Node`) kann zeitgleich als Server (eingehende Verbindungen) und als Client (ausgehende Verbindungen) agieren.
* **Kein TLS-Ballast im Code:** Entwickelt für gesicherte Umgebungen oder den Betrieb hinter Proxies (z. B. **Caddy**), die TLS-Terminierung deutlich effizienter handhaben.
* **Heartbeat & Time-Sync:** Integrierter Liveness-Check von der Client-Rolle aus. Der Empfänger antwortet automatisch mit `pong` und einem präzisen UTC-Timestamp.
* **Session-Token Authentication:** Integriertes Auth-System für Logins (Benutzername/Passwort) und nahtlose Reconnects via Session-Tokens.
* **Entkoppelte User-Verwaltung:** Über das `UserAuthenticator`-Interface kann jede beliebige Datenbank, LDAP oder Custom-Auth-Logik angedockt werden.
* **Strukturierte Fehler:** Vollständige Integration mit `nexutils/errors` (Fehler-Details werden im `data`-Feld von JSON-RPC übertragen).

---

## Architektur & Konzepte

### 1. Die Komponenten

Das Paket teilt sich in fünf klare Zuständigkeiten auf:

| Datei | Aufgabe |
| --- | --- |
| **`node.go`** | Haupt-Lifecycle-Manager. Startet den HTTP/WebSocket-Listener, stoppt das System und stellt ausgehende Verbindungen her (`ConnectToPeer`). |
| **`peer.go`** | Repräsentiert eine aktive WebSocket-Verbindung. Steuert Read/Write-Loops, hält den Auth-State und führt den Heartbeat aus. |
| **`router.go`** | Registriert RPC-Methoden (`RegisterHandler`), verwaltet die Session-Tokens und verteilt eingehende Anfragen (`dispatchLoop`). |
| **`protocol.go`** | JSON-RPC 2.0 Spezifikation (Requests, Responses, Errors) und Mapper für `nexutils/errors`. |
| **`options.go`** | Konfiguration für Timeouts, Ports und Heartbeat-Intervalle. |

### 2. Peer-to-Peer (P2P) Prinzip

Nachdem der initiale WebSocket-Handshake via HTTP GET vollzogen ist, schaltet die Verbindung auf das bidirektionale WebSocket-Protokoll um. Ab diesem Moment gibt es **keine feste Server/Client-Hierarchie mehr**: Beide Seiten können völlig asynchron JSON-RPC Requests senden und empfangen.

```
       [ Node A ]                                      [ Node B ]
(Port :8080 / Server)                           (Port :8081 / Server)
          ▲                                               │
          │────── ConnectToPeer("ws://nodeA/ws") ─────────│  (RoleOutbound)
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

### 2. Eigene User-Verwaltung einbinden (`UserAuthenticator`)

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

### 3. Geschützte Methoden schreiben

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



### 2. Reconnect nach Verbindungsabbruch (Token)

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


---

## Organizational & Standards

* **Copyright:** © 2026 Georg Hagn.
* **Namespace:** `codeberg.org/tiny-frameworks/nexutils/logging`
* **License:** Apache License, Version 2.0.

*GSF-nexutils/logging is an independent open-source project and is not affiliated with any corporation of a similar name.*

---

## Contact

If you have questions or feedback, feel free to reach out:

📧 *georghagn [at] tiny-frameworks.io*

---

