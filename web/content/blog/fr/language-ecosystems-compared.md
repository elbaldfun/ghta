---
title: "Le nombre n'est pas le poids : ce que 67 000 dépôts disent des écosystèmes de langages"
description: "Nous suivons chaque jour 67 000 dépôts GitHub très étoilés. Répartis par langage, celui qui compte le plus de projets n'est pas celui dont le projet typique est le plus fort."
date: 2026-07-21
tags: [données, écosystèmes]
translatedFrom: en
---

On compare habituellement les écosystèmes de langages en demandant lequel compte le plus de projets. La question est facile à trancher et faible pour décider : elle parle de **nombre**, pas de **poids**.

Nous suivons plus de 67 000 dépôts GitHub très étoilés et enregistrons chaque jour l'évolution de leurs étoiles. Répartir ce corpus par langage révèle plus utile qu'un simple décompte.

## Les chiffres bruts

Ce tableau est lu en direct depuis notre base à chaque chargement de page — ce n'est pas une capture d'écran :

```starrank:languages
limit: 10
```

## Trois points à relever

### 1. Le premier en nombre n'est pas premier en médiane

Python domine largement le nombre de dépôts, environ 1,4 fois le deuxième. Mais si l'on regarde la **médiane** d'étoiles — qui répond à « si je prends un projet au hasard dans ce langage, quelle popularité puis-je attendre ? » —, Python passe derrière Go et TypeScript.

C'est un signal d'**étendue**. Python couvre le calcul scientifique, le scraping, l'apprentissage automatique, l'automatisation : presque tout. Le revers de l'étendue est une très longue traîne, avec beaucoup de projets à peine au-dessus du seuil de suivi.

### 2. Go a le meilleur taux de réussite

Go compte environ un tiers des dépôts de Python, et pourtant la médiane la plus élevée de tous les langages présentés ici.

L'explication plausible est la **concentration de domaine**. Les projets populaires en Go se regroupent dans l'infrastructure cloud-native, le DevOps et l'outillage en ligne de commande — des champs dont les utilisateurs sont eux-mêmes des développeurs, donc des gens qui mettent des étoiles. Une grande part de la portée de Python touche des publics qui n'ouvrent jamais GitHub.

### 3. Total d'étoiles et médiane racontent deux histoires

JavaScript affiche un total d'étoiles élevé et une médiane relativement basse. Le total reflète une cohorte de projets très gros et très anciens : c'est de l'**histoire accumulée**. La médiane molle suggère qu'une part plus faible des projets récents parvient à percer.

TypeScript inverse la tendance : moins de dépôts que JavaScript mais une médiane supérieure, cohérent avec des projets récents qui choisissent TypeScript dès le premier jour.

## Le haut du classement

La composition compte autant que les décomptes. Voici les dépôts les plus étoilés que nous suivons actuellement :

```starrank:top-repos
limit: 10
```

Remarquez combien sont des **ressources d'apprentissage et des listes curées** plutôt que des logiciels exécutables. Ils accumulent des quantités énormes d'étoiles tout en disant peu de chose de l'écosystème d'ingénierie d'un langage ; les mêler aux frameworks et aux runtimes fausse toute comparaison.

## Ce que ces données ne peuvent pas dire

Toute analyse devrait énoncer ses propres limites :

- **L'échantillon est constitué de projets déjà populaires**, pas de tout GitHub. Les médianes ici sont des médianes *parmi des projets ayant déjà franchi la barre*, bien au-dessus du chiffre réel à l'échelle du site.
- **Les étoiles ne sont ni la qualité** ni l'usage. Elles approchent la visibilité auprès des développeurs. Beaucoup de bibliothèques critiques affichent bien moins d'étoiles que leur importance réelle.
- **L'attribution du langage suit la détection du langage principal de GitHub.** Un framework frontend peut se retrouver étiqueté de façon inattendue selon la répartition des octets de ses fichiers.
- **Les écosystèmes n'ont pas le même âge.** C et Java accumulent des décennies ; Rust a eu bien moins de temps. Comparer les totaux directement désavantage les plus jeunes.

## À retenir

Si vous choisissez une stack, une meilleure question que « quel langage a le plus de projets » est : **combien d'options matures existent dans la niche précise dont j'ai besoin**. Les décomptes globaux y aident étonnamment peu.

Le tableau ci-dessus se met à jour avec notre collecte quotidienne. Vous pouvez découper les mêmes données par langage et par catégorie dans le [classement](/).
