---
title: "Ein Drittel der meistgesternten GitHub-Repos liegt seit zwei Jahren still"
description: "20.435 der von uns verfolgten Repositories mit vielen Sternen haben seit über zwei Jahren keinen Push erhalten. Die naheliegenden Erklärungen — alles nur Awesome-Listen, die sind eben fertig — erklären weniger, als man denkt."
date: 2026-07-22
tags: [Daten, Wartung]
translatedFrom: en
---

Wir verfolgen über 67.000 GitHub-Repositories mit mindestens tausend Sternen und erfassen ihren Zustand täglich. Sortiert man sie danach, wie lange der letzte Push zurückliegt, ergibt sich eine unangenehme Zahl.

```starrank:staleness
```

Rund ein Drittel hat seit über zwei Jahren **keinen einzigen Push** erhalten. Weniger als die Hälfte wurde in den letzten drei Monaten angefasst.

Diese Zahl provoziert sofort zwei Einwände. Beide sind vernünftig. Keiner übersteht die Daten unbeschadet.

## Einwand 1: „Das sind doch alles Awesome-Listen"

Die Intuition: Kuratierte Listen, Buchsammlungen und Interview-Repos horten Sterne, ohne je einen Commit zu brauchen. Sie blähen den stillen Bereich auf und sagen nichts über Software aus.

Der Effekt ist real. Repositories **ohne erkannte Programmiersprache** machen 5,0 % der aktiv gepflegten Gruppe aus und 10,8 % der seit zwei Jahren kalten — exakt das Doppelte. Nimmt man dokumentartige Sprachen hinzu (Markdown, HTML, TeX, Jupyter), bleibt das Muster: 3,4 % gegenüber 6,0 %.

Zusammen ist das aber nur etwa ein Sechstel der kalten Gruppe. **Die anderen fünf Sechstel sind Repositories mit einer echten Programmiersprache** — tatsächliche Software, seit zwei Jahren unberührt, mit Tausenden Sternen darauf.

Die größten davon:

```starrank:stale-repos
limit: 10
```

Manches ist echtes Referenzmaterial. Anderes ist Software, die Leute weiterhin installieren.

## Einwand 2: „Still heißt nicht aufgegeben — gute Software wird fertig"

Dieser Einwand wiegt schwerer. Eine kleine, fokussierte Bibliothek, die ihr Problem sauber gelöst hat, braucht keine Commits. Betriebsamkeit ist keine Gesundheit, und ein stilles Repo ist womöglich schlicht ein **fertiges** Repo.

Wäre das die ganze Geschichte, müsste sich im Issue-Tracker eine Signatur zeigen. Ein aufgegebenes Projekt mit echten Nutzern sammelt Issues an, die niemand sichtet; ein wirklich fertiges Projekt zieht von vornherein wenige an. Der auf die Publikumsgröße normierte Rückstau sollte zwischen beiden Gruppen also deutlich verschieden aussehen.

Tut er nicht. Schau auf die letzte Spalte der Tabelle: **offene Issues je tausend Sterne liegen in jeder Gruppe zwischen 9 und 11,5**. Aktiv gepflegte Projekte tragen einen minimal **höheren** normierten Rückstau als die seit zwei Jahren kalten.

Das Ergebnis hat uns überrascht — wir haben die Spalte gebaut, damit sie die Gruppen trennt, und sie weigerte sich.

## Was diese flache Linie vermutlich bedeutet

Die plausibelste Lesart: Aufgabe ist wechselseitig. Projekte verstummen selten, während Nutzer weiter auf den Issue-Tracker einhämmern. Die Aufmerksamkeit geht von beiden Seiten gleichzeitig — wer pflegt, hört auf zu pushen, und wer Issues gemeldet hätte, ist längst zu etwas anderem gewechselt.

Das ist weniger dramatisch als „Tausende vernachlässigte Projekte mit wütenden Nutzern". Es ist zugleich die schlechtere Nachricht für alle, die Abhängigkeiten nach Sternen auswählen. Ein Repository kann gleichzeitig hoch gesternt, sichtbar still und **von außen nicht erkennbar kaputt** sein — weil diejenigen, die sich beschwert hätten, wortlos gegangen sind.

Man sollte das nicht überdehnen. Sichtbar ist **eine einzige aktuelle Momentaufnahme** der offenen Issues, und GitHubs Zahl **zählt Pull Requests mit**. Ob Issues massenhaft geschlossen wurden, ob jemand den Tracker abgeschaltet hat, wie sich der Rückstau über die Zeit bewegte — all das sehen wir nicht. Die flache Linie ist mit wechselseitiger Aufgabe **vereinbar**; sie beweist sie nicht.

## Die praktische Fassung

Die Sternzahl hält fest, wie viele Leute ein Projekt einmal für lesezeichenwürdig hielten. Sie sagt nichts darüber, ob es noch jemand pflegt — und nach den Issue-Daten auch nicht viel darüber, ob es noch jemand benutzt.

Bevor du eine Abhängigkeit wegen ihrer Sternzahl übernimmst, **sieh dir das Datum des letzten Pushes an**. Es ist einen Klick entfernt und widerspricht der Sternzahl in etwa einem Drittel der Fälle.

## Was diese Daten nicht sagen können

- **Die Stichprobe sind Repositories, die 1.000 Sterne bereits überschritten haben**, nicht ganz GitHub. Nichts hiervon beschreibt typische Repositories.
- **`pushedAt` zählt Pushes auf beliebige Branches**, auch automatisierte Commits. Es ist ein Lebenszeichen, kein Maß für sinnvolle Arbeit.
- **Die Zahl offener Issues enthält Pull Requests**, und wir haben eine einzelne aktuelle Momentaufnahme statt eines Verlaufs.
- **Die Sprachzuordnung folgt GitHubs eigener Erkennung**, die nach Datei-Bytes entscheidet und ein Projekt unerwartet einsortieren kann.
- Weitere **4.123** Repositories haben kein brauchbares Push-Datum und fallen aus allen obigen Gruppen heraus.

Die Tabellen aktualisieren sich mit unserem täglichen Crawl. Denselben Korpus kannst du im [Ranking](/) selbst sortieren.
