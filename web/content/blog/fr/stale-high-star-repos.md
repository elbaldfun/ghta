---
title: "Un tiers des dépôts GitHub les plus étoilés sont figés depuis deux ans"
description: "20 435 des dépôts très étoilés que nous suivons n'ont reçu aucun push depuis plus de deux ans. Les explications évidentes — ce ne sont que des listes curées, ils sont simplement terminés — expliquent moins qu'on ne le croit."
date: 2026-07-22
tags: [données, maintenance]
translatedFrom: en
---

Nous suivons plus de 67 000 dépôts GitHub comptant au moins mille étoiles et enregistrons leur état chaque jour. Les trier selon l'ancienneté du dernier push donne un chiffre inconfortable.

```starrank:staleness
```

Environ un tiers **n'a reçu aucun push depuis plus de deux ans**. Moins de la moitié a été touchée au cours des trois derniers mois.

Ce chiffre appelle deux objections immédiates. Les deux sont raisonnables. Aucune ne sort indemne des données.

## Objection 1 : « Ce ne sont que des awesome-lists »

L'intuition : les listes curées, les collections de livres et les dépôts de préparation aux entretiens accumulent des étoiles sans jamais nécessiter de commit. Ils gonflent le groupe figé sans rien dire du logiciel.

L'effet est réel. Les dépôts **sans langage de programmation détecté** représentent 5,0 % du groupe activement maintenu et 10,8 % du groupe froid depuis deux ans — exactement le double. En ajoutant les langages documentaires (Markdown, HTML, TeX, Jupyter), le motif tient : 3,4 % contre 6,0 %.

Mais ensemble, cela fait environ un sixième du groupe froid. **Les cinq autres sixièmes sont des dépôts dotés d'un vrai langage de programmation** — du logiciel bien réel, intouché depuis deux ans, avec des milliers d'étoiles pointées dessus.

Voici les plus gros :

```starrank:stale-repos
limit: 10
```

Certains sont effectivement du matériel de référence. D'autres sont des logiciels que les gens installent encore.

## Objection 2 : « Figé n'est pas abandonné — un bon logiciel se termine »

Celle-ci est plus solide. Une bibliothèque petite et ciblée qui a résolu son problème correctement n'a pas besoin de commits. L'agitation n'est pas la santé, et un dépôt silencieux est peut-être simplement un dépôt **terminé**.

Si c'était toute l'histoire, on s'attendrait toutefois à une signature dans le suivi d'issues. Un projet abandonné ayant de vrais utilisateurs accumule des issues que personne ne trie ; un projet réellement terminé n'en attire pas beaucoup au départ. L'arriéré, normalisé par la taille de l'audience, devrait donc différer nettement entre les deux groupes.

Ce n'est pas le cas. Regardez la dernière colonne du tableau : **les issues ouvertes pour mille étoiles se situent entre 9 et 11,5 dans tous les groupes**. Les projets activement maintenus portent un arriéré normalisé légèrement **supérieur** à celui des projets froids depuis deux ans.

Ce résultat nous a surpris : nous avions construit cette colonne pour séparer les groupes, et elle a refusé de le faire.

## Ce que cette ligne plate signifie probablement

La lecture la plus plausible est que l'abandon est mutuel. Un projet devient rarement silencieux pendant que les utilisateurs continuent de marteler le suivi d'issues. L'attention part des deux côtés en même temps : le mainteneur cesse de publier, et ceux qui auraient ouvert des issues sont déjà passés à autre chose.

C'est une histoire moins spectaculaire que « des milliers de projets délaissés avec des utilisateurs furieux ». C'est aussi une plus mauvaise nouvelle pour qui choisit ses dépendances au nombre d'étoiles. Un dépôt peut être à la fois très étoilé, visiblement immobile et **sans avoir l'air cassé de l'extérieur** — parce que ceux qui se seraient plaints sont partis sans rien dire.

Il faut se garder d'aller trop loin. Nous voyons **un unique instantané actuel** du nombre d'issues ouvertes, et le chiffre de GitHub **inclut les pull requests**. Nous ignorons si des issues ont été fermées en masse, si le mainteneur a désactivé le suivi, ou comment l'arriéré a évolué. La ligne plate est **compatible** avec l'abandon mutuel ; elle ne le prouve pas.

## La version pratique

Le nombre d'étoiles enregistre combien de personnes ont un jour jugé un projet digne d'un signet. Il ne dit rien de savoir si quelqu'un le maintient encore — et, au vu des données d'issues, pas grand-chose non plus sur le fait que quelqu'un l'utilise encore.

Avant d'adopter une dépendance sur la foi de ses étoiles, **regardez la date du dernier push**. Elle est à un clic et contredit le nombre d'étoiles environ une fois sur trois.

## Ce que ces données ne peuvent pas dire

- **L'échantillon est constitué de dépôts ayant déjà dépassé 1 000 étoiles**, pas de tout GitHub. Rien ici ne décrit des dépôts typiques.
- **`pushedAt` compte les pushes sur n'importe quelle branche**, y compris les commits automatisés. C'est un signe de vie, pas une mesure de travail utile.
- **Le nombre d'issues ouvertes inclut les pull requests**, et nous disposons d'un seul instantané actuel, pas d'un historique.
- **L'attribution du langage suit la détection de GitHub**, qui tranche selon les octets des fichiers et peut étiqueter un projet de façon inattendue.
- **4 123** dépôts supplémentaires n'ont pas de date de push exploitable et sortent de tous les groupes ci-dessus.

Les tableaux se mettent à jour avec notre collecte quotidienne. Vous pouvez trier le même corpus vous-même dans le [classement](/).
