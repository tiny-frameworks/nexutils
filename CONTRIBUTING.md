
<sup> 🌍 **Sprache:** [🇩🇪 Deutsch](#beitragen) | [🇬🇧 English](#contributing)</sup>

---

# Contributing 

## Thank you

First of all: **Thank you** for taking the time to contribute to the **GSF Suite**.
Open source lives from people who share their knowledge, time, and experience.

This document outlines a few guidelines intended to keep the project **clear, stable, and maintainable in the long term**.

---

## 1. Reporting Bugs

Before opening a new issue, please check:

* whether the problem has already been reported
* whether you are using the **latest available version** of the relevant module

When reporting a bug, the following information is very helpful:

* version(s) used
* steps to reproduce the issue
* expected behavior vs. actual behavior
* optional: a **minimal code example**

> **Note on security issues:**
> If you believe the issue may be **security-related**, please **do not open a public issue**,
> but follow the instructions in `SECURITY.md`.

---

## 2. Pull Requests (Contributing Code)

We welcome pull requests — whether they fix bugs, improve existing code, or add new features.

To help us review and merge your PR efficiently, please follow these guidelines:

1. **Fork & Branch**
   Fork the repository and work in a dedicated feature branch:

   ```bash
   git checkout -b feature/my-feature
   ```

2. **Coding Style**
   Please follow the existing coding style and design principles of the respective module.

3. **Tests**

   * New functionality should be accompanied by appropriate tests
   * Bug fixes should include a regression test when reasonable

4. **License Header**
   New source files must include the correct SPDX license header:

   ```go
   // Copyright 2026 Georg Hagn (tiny-frameworks)
   // SPDX-License-Identifier: Apache-2.0
   ```

5. **Security-related Changes**
   If your pull request addresses a potential security vulnerability,
   please refer to `SECURITY.md` and consider submitting the PR as a **draft** first.

---

## 3. Legal & Licensing

By submitting a pull request, you confirm that:

1. you are the original author of the contributed code **or** have the right to submit it
2. your contribution may be published under the **Apache License 2.0**

This project follows the **"Inbound = Outbound"** principle:

* No separate Contributor License Agreement (CLA) is required
* All contributions are licensed under the same terms as the project itself

---

## 4. Philosophy

The GSF Suite deliberately follows a **Tiny / Simple philosophy**:

* minimal dependencies
* explicit APIs over magic
* small, clearly scoped modules
* predictability over feature richness

Contributions should respect and align with these principles.

---

Thank you for your contribution!
**We** appreciate constructive discussions, clean contributions, and shared evolution.

---


---

# Beitragen

## Danke

Erstmal: **Vielen Dank**, dass du dir die Zeit nimmst, zur **GSF‑Suite** beizutragen!
Open Source lebt von Menschen, die ihr Wissen, ihre Zeit und ihre Erfahrung teilen.

Dieses Dokument beschreibt einige Leitlinien, die helfen sollen, das Projekt **übersichtlich, stabil und langfristig wartbar** zu halten.

---

## 1. Bugs melden

Bevor du ein neues Issue erstellst, prüfe bitte:

* ob das Problem bereits gemeldet wurde
* ob du die **neueste verfügbare Version** des jeweiligen Moduls verwendest

Wenn du ein Bug‑Report erstellst, helfen uns folgende Informationen sehr:

* verwendete Version(en)
* Schritte zur Reproduktion
* erwartetes Verhalten vs. tatsächliches Verhalten
* optional: ein **minimales Code‑Beispiel**

> **Hinweis zu Sicherheitslücken:**
> Wenn du vermutest, dass es sich um ein **sicherheitsrelevantes Problem** handelt,
> **bitte kein öffentliches Issue eröffnen**, sondern die Hinweise in der `SECURITY.md` beachten.

---

## 2. Pull Requests (Code beitragen)

Wir freuen uns über Pull Requests – egal ob Bugfix, Verbesserung oder neues Feature.

Damit dein PR gut nachvollziehbar ist und zügig geprüft werden kann, beachte bitte:

1. **Fork & Branch**
   Erstelle einen Fork des Repositories und arbeite in einem eigenen Feature‑Branch:

   ```bash
   git checkout -b feature/mein-feature
   ```

2. **Coding Style**
   Halte dich bitte an den bestehenden Code‑Stil und die Design‑Prinzipien des jeweiligen Moduls.

3. **Tests**

   * Neue Funktionalität sollte durch passende Tests begleitet werden
   * Bugfixes sollten – wenn sinnvoll – einen Regressionstest enthalten

4. **Lizenz‑Header**
   Neue Dateien müssen den korrekten SPDX‑Lizenz‑Header enthalten:

   ```go
   // Copyright 2026 Georg Hagn (tiny-frameworks)
   // SPDX-License-Identifier: Apache-2.0
   ```

5. **Security‑relevante Änderungen**
   Wenn dein Pull Request eine potenzielle Sicherheitslücke betrifft,
   orientiere dich bitte an der `SECURITY.md` und reiche den PR ggf. zunächst als **Draft** ein.

---

## 3. Rechtliches & Lizenzierung

Durch das Einreichen eines Pull Requests bestätigst du, dass:

1. du der Urheber des beigetragenen Codes bist **oder** die notwendigen Rechte besitzt
2. dein Beitrag unter der **Apache License 2.0** veröffentlicht werden darf

Dieses Projekt folgt dem Prinzip **„Inbound = Outbound“**:

* Es wird **kein separater Contributor License Agreement (CLA)** benötigt
* Alle Beiträge stehen automatisch unter derselben Lizenz wie das Projekt selbst

---

## 4. Philosophie

Die GSF‑Suite folgt bewusst einer **Tiny / Simple‑Philosophie**:

* minimale Abhängigkeiten
* explizite APIs statt Magie
* kleine, klar abgegrenzte Module
* Vorhersehbarkeit vor Feature‑Reichtum

Beiträge sollten diese Grundhaltung respektieren.

---

Vielen Dank für deine Unterstützung!
**Wir** freuen uns über konstruktive Diskussionen, saubere Beiträge und gemeinsame Weiterentwicklung.

