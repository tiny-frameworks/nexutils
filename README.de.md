
<sup>🌍 **Language:** 🇬🇧 [Englisch →](README.md)</sup>

---
|[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](./LICENSE)| |
|----|----|
|![GSF-Suite-Logo](logo-gsf.png)| ***GSF-nexutils***<br>Eine performante, modulare Sammlung essenzieller Go-Utilities für robuste Microservices, CLI-Engines und verteilte Peer-to-Peer-Anwendungen|
<sup>***GSF*** steht für ***Go Small Frameworks*** — minimalistische Tools für robuste Applicationen.</sup>

---

# GSF-nexutils

GSF-nexutils ist eine hochperformante, modulare Sammlung essenzieller Go-Utilities, die für robuste Microservices, CLI-Engines und verteilte Peer-to-Peer-Anwendungen entwickelt wurde. Gebaut nach einer strikten **Zero-External-Dependency**-Philosophie (keine externen Abhängigkeiten), bietet es standardisierte Grundlagen für Fehlerbehandlung, Thread-/Prozess-Synchronisation, In-Memory-Caching und strukturiertes Logging.

---

## Modulübersicht

| Modul | Unterpaket-Pfad | Beschreibung |
| --- | --- | --- |
| **`cache`** | `nexutils/cache` | Thread-sicherer LRU-Cache mit TTL, Hintergrund-Cleanup und JSON-Persistenz. |
| **`errors`** | `nexutils/errors` | Domain-driven Fehlerbehandlung mit Fehlercodes, Call-Paths und OS Exit-Code Mapping. |
| **`lockingwriter`** | `nexutils/lockingwriter` | Prozess-sicherer `io.Writer` mittels `.LOCK`-Dateien inklusive PID & Timestamp-Erkennung verwaister Locks. |
| **`logging`** | `nexutils/logging` | `slog`-basierter strukturierter Logger mit nativer Unterstützung für `nexutils/errors`. |

---

## Design-Prinzipien

1. **Zero External Dependencies:** Stützt sich rein auf die Go-Standardbibliothek (`sync`, `time`, `os`, `log/slog`, `errors`).
2. **Modulare Architektur:** Jedes Unterpaket kann unabhängig importiert werden, ohne unnötigen Code in dein Binary zu ziehen.
3. **Fail-Fast & Self-Healing:** Integrierte Schutzmechanismen wie die Wiederherstellung verwaister Locks (Stale-Lock-Recovery), begrenzte LRU-Speicherkapazitäten und typsichere Fehlerketten.
4. **Developer Experience:** Erstklassige Unterstützung für Go-`Example`-Tests und klare API-Signaturen.

---

## Installation

Importiere nur die Module, die du in deinem Go-Code benötigst:

```bash
go get codeberg.org/tiny-frameworks/nexutils

```

```go
import (
    "codeberg.org/tiny-frameworks/nexutils/cache"
    "codeberg.org/tiny-frameworks/nexutils/errors"
    "codeberg.org/tiny-frameworks/nexutils/lockwriter"
    "codeberg.org/tiny-frameworks/nexutils/logging"
)

```

---

## Projektstruktur

```text
nexutils/
├── cache/            # LRU-Cache mit TTL & JSON-Speicherung
├── errors/           # Domain-Fehlercodes & Exit-Code Mapping
├── lockwriter/       # Thread- & prozess-sicherer Datei-Writer
├── logging/          # Strukturierter JSON/Text-Logger
├── go.mod
├── LICENSE
└── README.md

```

---

## Tests ausführen

Führe die Unit-Tests über alle Utility-Module hinweg gleichzeitig aus:

```bash
go test -v ./...

```

---

## Organisation & Standards

* **Copyright:** © 2026 Georg Hagn.
* **Repository:** `codeberg.org/tiny-frameworks/nexutils`
* **Lizenz:** Apache License, Version 2.0.

*GSF-nexutils ist ein unabhängiges Open-Source-Projekt und steht in keiner Verbindung zu Unternehmen mit ähnlichem Namen.*

---

## Kontakt & Support

Für Anfragen, architektonische Diskussionen oder Sicherheitsfragen wende dich bitte an:

📧 **georghagn [at] tiny-frameworks.io**

---

