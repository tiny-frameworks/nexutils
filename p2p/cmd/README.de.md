
# Microservice Communication Demo Suite

Dieses Verzeichnis enthält Beispiel-Microservices, die das Architekturmuster und das Resilienz-Verhalten des `nexutils/p2p/rpc`-Pakets demonstrieren.

---

## Architektur & Service-Rollen

```
                   ┌─────────────────────────┐
                   │     payment-service     │
                   │   (WebSocket Server)    │
                   └────────────▲────────────┘
                                │
            ┌───────────────────┴──────────────────┐
            │                                      │
  [Statische Verbindung]                [   Managed Client    ]
       Ad-hoc Call                      Auto-Reconnect & Re-Auth
            │                                      │
 ┌──────────┴──────────┐                ┌──────────┴──────────┐
 │    order-service    │                │   secret-service    │
 └─────────────────────┘                └─────────────────────┘

```

### 1. `payment-service` (Der Kernel / Server)
* **Rolle:** Zentraler Zahlungsdienst.
* **Verhalten:** Verifiziert Credentials, stellt Session-Tokens aus und verarbeitet Zahlungsaufträge (`payment.process`). Sendet periodisch System-Broadcasts an alle verbundenen Partner.

### 2. `secret-service` (Entkoppelter, hochverfügbarer Worker)
* **Rolle:** Baut eine dauerhafte, autonome Verbindung zum `payment-service` auf.
* **Architektur:** Verwendet `ConnectWithAutoReconnect` & `ManagedClient`.
* **Resilienz:** 
  * Übersteht Abstürze und Neustarts des `payment-service` ohne eigenen Abbruch.
  * Versucht automatisch die Wiederverbindung per Token. 
  * Fällt bei Token-Ablauf nahtlos auf `Username/Password` zurück, holt ein neues Token und setzt die Arbeit fort.

### 3. `order-service` (Kompakter, direkter Client)
* **Rolle:** Führt fokussierte Ad-hoc-Transaktionen aus.
* **Architektur:** Verwendet `ConnectToPeer` (statische Direktverbindung).
* **Resilienz:** Bei einem Verbindungsabbruch schlägt der Call sofort fehl. Die Goroutine beendet sich geordnet oder bricht den Prozess über den Application-Context ab (`Graceful Shutdown`).

---

## Demo-Szenarien & Ausführung

Starte die Services in separaten Terminal-Fenstern:

```bash
# Terminal 1: Zentralen Payment-Service starten
go run cmd/services/payment-service/main.go

# Terminal 2: Resilientem Worker starten
go run cmd/services/secret-service/main.go

# Terminal 3: Statischen Order-Service starten
go run cmd/services/order-service/main.go

```

### Resilienz-Test (Re-Auth & Backoff nachweisen)

1. Starte `payment-service` und `secret-service`.
2. **Töte den `payment-service**` mit `Strg + C`.
3. Beobachte die Logs im `secret-service`:
* Der Client wechselt in den Zustand `reconnecting...`.
* Ticker-Aufrufe werden abgefangen, ohne zu crashen.
* Der Reconnect-Loop startet mit Exponential Backoff (1s, 2s, 4s...).


4. **Starte den `payment-service` neu.**
5. Beobachte die automatisierte Wiederherstellung:
* Client stellt TCP-Verbindung her.
* Token-Auth schlägt fehl (Server-Speicher war leer).
* Fallback greift auf Credentials zurück.
* Neues Token wird gespeichert, Status wird `Ready`.
* Die Transaktionen laufen nahtlos weiter!


