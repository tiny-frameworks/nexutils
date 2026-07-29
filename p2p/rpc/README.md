

# Tiny-P2P RPC Framework

Ein leichtgewichtiges, bidirektionales RPC-Framework für Go, das auf WebSockets basiert und echtes Peer-to-Peer Verhalten in Microservice-Architekturen ermöglicht.

## Architektur-Übersicht

Das Framework ist in zwei Schichten unterteilt:
- **Transport-Layer:** Abstrahiert die Netzwerk-Verbindung (WebSockets).
- **RPC-Layer:** Verwaltet die JSON-RPC Logik, IDs und Handler.



## Features

- **Symmetrisch:** Beide Seiten können gleichzeitig Anfragen senden und empfangen.
- **Resilient:** Integrierter Exponential-Backoff für automatische Wiederverbindung.
- **Standardkonform:** Implementiert JSON-RPC 2.0 Spezifikation inklusive des `data`-Feldes für detaillierte Fehlermeldungen.
- **Typsicher:** Nutzung von Go Generics für Parameter-Binding.

## Schnellstart

### 1. Verbindung herstellen
```go
provider := &transport.WSProvider{}
// Als Client (mit Reconnect-Logik)
peer := rpc.NewPeer(nil, provider, "ws://localhost:8080/ws")
go peer.Listen(ctx)

```

### 2. Methoden aufrufen

```go
result, err := peer.Call(ctx, "math.add", []int{5, 10})

```

### 3. Auf Benachrichtigungen reagieren

```go
peer.Register("system.alert", func(ctx context.Context, p json.RawMessage) (any, error) {
    msg, _ := rpc.Bind[string](p)
    fmt.Println("Alert:", msg)
    return nil, nil
})

```

## Installation (Lokal für Tests)

Um diese Library in einem anderen Projekt zu nutzen, verwende die `replace`-Direktive in deiner `go.mod`:

```bash
go mod edit -replace tiny-p2p=../path/to/tiny-lib

```



