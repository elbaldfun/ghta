---
title: "Anzahl ist nicht Gewicht: Was 67.000 Repositories über Sprach-Ökosysteme verraten"
description: "Wir verfolgen täglich 67.000 GitHub-Repositories mit vielen Sternen. Nach Sprache aufgeschlüsselt zeigt sich: Die Sprache mit den meisten Projekten hat nicht das stärkste typische Projekt."
date: 2026-07-21
tags: [Daten, Ökosysteme]
translatedFrom: en
---

Sprach-Ökosysteme werden üblicherweise über die Frage verglichen, welche Sprache die meisten Projekte hat. Sie ist leicht zu beantworten und taugt schlecht als Entscheidungsgrundlage: Sie sagt etwas über **Anzahl**, nicht über **Gewicht**.

Wir verfolgen mehr als 67.000 GitHub-Repositories mit vielen Sternen und protokollieren täglich, wie sich ihre Sterne bewegen. Schlüsselt man diesen Korpus nach Sprache auf, zeigt sich Nützlicheres als eine Kopfzahl.

## Die Rohdaten

Diese Tabelle wird bei jedem Seitenaufruf live aus unserer Datenbank gelesen — sie ist kein Screenshot:

```starrank:languages
limit: 10
```

## Drei bemerkenswerte Punkte

### 1. Der Erste nach Anzahl ist nicht der Erste nach Median

Python liegt bei der Zahl der Repositories deutlich vorn, etwa beim 1,4-Fachen des Zweitplatzierten. Betrachtet man aber den **Median** der Sterne — also die Antwort auf „Wenn ich zufällig ein Projekt dieser Sprache greife, wie beliebt ist es voraussichtlich?" —, liegt Python hinter Go und TypeScript.

Das ist ein Zeichen von **Breite**. Python deckt wissenschaftliches Rechnen, Scraping, maschinelles Lernen und Automatisierung ab — praktisch alles. Die Kehrseite der Breite ist ein sehr langer Schwanz mit vielen Projekten knapp über der Erfassungsschwelle.

### 2. Go hat die höchste Trefferquote

Go hat rund ein Drittel der Repositories von Python und dennoch den höchsten Median aller hier gezeigten Sprachen.

Die plausible Erklärung ist **Domänenkonzentration**. Gos beliebte Projekte ballen sich in Cloud-native-Infrastruktur, DevOps und CLI-Werkzeugen — Feldern, deren Nutzer selbst Entwickler sind und also Sterne vergeben. Ein großer Teil von Pythons Reichweite geht an Zielgruppen, die GitHub nie öffnen.

### 3. Gesamtsterne und Median erzählen Verschiedenes

JavaScript hat eine hohe Gesamtzahl an Sternen bei vergleichsweise niedrigem Median. Die Summe spiegelt eine Kohorte sehr großer, sehr alter Projekte — das ist **angesammelte Geschichte**. Der schwache Median deutet darauf hin, dass ein kleinerer Anteil neuerer Projekte durchbricht.

TypeScript kehrt das um: weniger Repositories als JavaScript, aber ein höherer Median — passend dazu, dass neue Projekte von Tag eins an TypeScript wählen.

## Die Spitze der Liste

Die Zusammensetzung zählt ebenso viel wie die Anzahl. Dies sind die Repositories mit den meisten Sternen, die wir derzeit verfolgen:

```starrank:top-repos
limit: 10
```

Auffällig ist, wie viele davon **Lernressourcen und kuratierte Listen** sind statt lauffähiger Software. Sie sammeln enorme Sternzahlen, sagen aber wenig über das Engineering-Ökosystem einer Sprache aus — sie mit Frameworks und Runtimes in einen Topf zu werfen, verzerrt jeden Vergleich.

## Was diese Daten nicht sagen können

Jede Analyse sollte ihre eigenen Grenzen benennen:

- **Die Stichprobe besteht aus bereits populären Projekten**, nicht aus ganz GitHub. Die Mediane hier sind Mediane *unter Projekten, die die Hürde schon genommen haben* — weit über dem tatsächlichen Wert der gesamten Plattform.
- **Sterne sind weder Qualität** noch Nutzung. Sie nähern die Sichtbarkeit unter Entwicklern an. Viele geschäftskritische Bibliotheken haben deutlich weniger Sterne, als ihrer Bedeutung entspräche.
- **Die Sprachzuordnung folgt GitHubs Erkennung der Hauptsprache.** Ein Frontend-Framework kann je nach Byte-Verteilung seiner Dateien unerwartet einsortiert werden.
- **Ökosysteme sind unterschiedlich alt.** C und Java haben Jahrzehnte an Ansammlung, Rust hatte weit weniger Zeit. Summen direkt zu vergleichen, benachteiligt die jüngeren.

## Fazit

Wer einen Stack wählt, fährt mit einer besseren Frage als „welche Sprache hat mehr Projekte": **Wie viele ausgereifte Optionen gibt es in genau der Nische, die ich brauche?** Globale Zahlen helfen dabei erstaunlich wenig.

Die Tabelle oben aktualisiert sich mit unserem täglichen Crawl. Dieselben Daten lassen sich im [Ranking](/) nach Sprache und Kategorie selbst zerlegen.
