
<sup> 🌍 **Sprache:** [🇩🇪 Deutsch](#sicherheitsrichtlinie) | [🇬🇧 English](#security_policy)</sup>

---

# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability, please **do not open a public issue**, so that potential gaps cannot be exploited before they are fixed

Instead, report it responsibly:

* Instead, please send directly an E-Mail to: **security@tiny-frameworks.io**
* Alternatively: Send an email to the address in the README
* Optional: open a draft pull request describing the issue and proposed fix

Please include:

* A clear description of the vulnerability
* Steps to reproduce (if applicable)
* Affected module(s)
* Any suggested mitigation or fix

For general bug reports, feature suggestions, or questions, please use the regular codeberg.org issues according to the instructions in `CONTRIBUTING.md`.

I run this project in my spare time, but I strive to respond to reports as quickly as possible.

---

## Scope

GSF focuses on:

* In-process libraries (logging, scheduling, file rotation)
* Minimal dependencies (standard library preferred)

Out of scope:

* Network security (TLS, authentication, authorization)
* OS-level hardening
* Application-level security policies

---

## Disclosure Policy

Reported vulnerabilities will be:

1. Reviewed as soon as reasonably possible
2. Fixed in a private branch if necessary
3. Released publicly once a fix is available

There is no formal CVE process at this stage.

---

## Final Note

GSF aims to be **simple, predictable, and transparent**.

If something looks unsafe, ambiguous, or surprising: **please report it**

Security arises from shared awareness.

---

# Sicherheitsrichtlinie 

## Melden von Sicherheitslücken

Wenn du eine potenzielle Sicherheitslücke entdeckst, **bitte kein öffentliches Issue eröffnen**.

Stattdessen bitten wir um eine verantwortungsvolle Meldung über einen der folgenden Wege:

* Kontaktaufnahme mit dem Maintainer über GitHub (private Nachricht)
* Alternativ: Schicke eine Email an die Adresse im README
* Optional: ein **Draft Pull Request**, der das Problem und einen möglichen Fix beschreibt

Bitte gib dabei möglichst an:

* Eine klare Beschreibung der Schwachstelle
* Schritte zur Reproduktion (falls zutreffend)
* Betroffene(s) Modul(e)
* Mögliche Gegenmaßnahmen oder einen Fix-Vorschlag

Für allgemeine Bugs, Feature-Vorschläge oder Fragen nutze bitte die regulären GitHub Issues
gemäß den Hinweisen in `CONTRIBUTING.md`.

---

## Geltungsbereich (Scope)

GSF konzentriert sich bewusst auf:

* **In-Process Libraries** (z. B. Logging, Scheduling, File Rotation)
* **Minimale Abhängigkeiten** (bevorzugt Go-Standardbibliothek)

Nicht im Fokus dieses Projekts sind:

* Netzwerksicherheit (TLS, Authentifizierung, Autorisierung)
* Betriebssystem-Härtung
* Anwendungsspezifische Security-Policies

---

## Umgang mit gemeldeten Sicherheitsproblemen

Gemeldete Sicherheitslücken werden:

1. so zeitnah wie möglich geprüft
2. bei Bedarf in einem privaten Branch behoben
3. öffentlich veröffentlicht, sobald ein Fix verfügbar ist

Aktuell gibt es **keinen formalen CVE-Prozess**.

---

## Abschließende Bemerkung

GSF verfolgt das Ziel, **einfach, vorhersehbar und transparent** zu sein.

Wenn dir etwas unsicher, unklar oder überraschend erscheint:
👉 **Bitte melde es.**

Sicherheit entsteht durch gemeinsame Aufmerksamkeit.

---
