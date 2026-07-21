---
title: "Un tercio de los repos más estrellados de GitHub lleva dos años frío"
description: "20.435 de los repositorios con muchas estrellas que seguimos no reciben un push desde hace más de dos años. Las explicaciones obvias —que son todos listas curadas, que simplemente están terminados— resultan explicar menos de lo que parece."
date: 2026-07-22
tags: [datos, mantenimiento]
translatedFrom: en
---

Seguimos más de 67.000 repositorios de GitHub con mil estrellas o más y registramos su estado a diario. Ordenarlos por cuánto hace desde el último push produce un número incómodo.

```starrank:staleness
```

Aproximadamente un tercio **no ha recibido un solo push en más de dos años**. Menos de la mitad se ha tocado en los últimos tres meses.

Ese número invita a dos objeciones inmediatas. Ambas son razonables. Ninguna sobrevive intacta a los datos.

## Objeción 1: «Todo eso son awesome-lists»

La intuición es que las listas curadas, las colecciones de libros y los repos de preparación de entrevistas acumulan estrellas sin necesitar nunca un commit. Inflan el grupo inactivo sin decirnos nada sobre software.

Es un efecto real. Los repositorios **sin lenguaje de programación detectado** son el 5,0% del grupo mantenido activamente y el 10,8% del grupo frío de dos años: exactamente el doble. Si añadimos los lenguajes de documentación (Markdown, HTML, TeX, Jupyter), el patrón se mantiene: 3,4% frente a 6,0%.

Pero juntos suman alrededor de una sexta parte del grupo frío. **Las otras cinco sextas partes son repositorios con un lenguaje de programación real**: software de verdad, sin tocar durante dos años, con miles de estrellas apuntándole.

Estos son los mayores:

```starrank:stale-repos
limit: 10
```

Algunos son material de referencia genuino. Otros son software que la gente sigue instalando.

## Objeción 2: «Inactivo no es abandonado; el buen software se termina»

Esta es más fuerte. Una biblioteca pequeña y enfocada que resolvió bien su problema no necesita commits. La agitación no es salud, y un repo silencioso puede estar simplemente **terminado**.

Si esa fuera toda la historia, sin embargo, esperaríamos una huella en el gestor de incidencias. Un proyecto abandonado con usuarios reales acumula issues que nadie tría; un proyecto realmente terminado no atrae muchas de entrada. Así que el atraso, normalizado por tamaño de audiencia, debería verse muy distinto entre ambos grupos.

No es así. Mira la última columna de la tabla: **las issues abiertas por cada mil estrellas se sitúan entre 9 y 11,5 en todos los grupos**. Los proyectos mantenidos activamente cargan un atraso normalizado ligeramente **mayor** que los fríos de dos años.

El resultado nos sorprendió: construimos esa columna esperando que separase los grupos, y se negó a hacerlo.

## Qué significa probablemente esa línea plana

La lectura más plausible es que el abandono es mutuo. Los proyectos rara vez se quedan en silencio mientras los usuarios siguen aporreando el gestor de incidencias. La atención se va por ambos lados a la vez: quien mantiene deja de publicar, y quienes habrían abierto issues ya se pasaron a otra cosa.

Es una historia menos dramática que «miles de proyectos desatendidos con usuarios furiosos». También es peor noticia para quien usa las estrellas de GitHub para elegir dependencias. Un repositorio puede estar a la vez muy estrellado, visiblemente quieto y **sin parecer roto desde fuera**, porque quienes se habrían quejado se marcharon sin decir nada.

Conviene no llevar esto demasiado lejos. Lo que vemos es **una única instantánea actual** del recuento de issues abiertas, y el número de GitHub **mezcla pull requests con issues**. No sabemos si se cerraron issues en masa, si quien mantiene desactivó el gestor, ni cómo evolucionó el atraso. La línea plana es **compatible** con el abandono mutuo; no lo demuestra.

## La versión práctica

El recuento de estrellas registra cuánta gente pensó alguna vez que un proyecto merecía un marcador. No dice nada sobre si alguien lo mantiene todavía y —a juzgar por los datos de issues— tampoco mucho sobre si alguien lo usa todavía.

Antes de adoptar una dependencia por sus estrellas, **mira la fecha del último push**. Está a un clic y discrepa del recuento de estrellas aproximadamente un tercio de las veces.

## Lo que estos datos no pueden decirte

- **La muestra son repositorios que ya superaron las 1.000 estrellas**, no todo GitHub. Nada de esto describe repositorios típicos.
- **`pushedAt` cuenta pushes a cualquier rama**, incluidos commits automatizados. Es una señal de vida, no una medida de trabajo significativo.
- **El recuento de issues abiertas incluye pull requests**, y tenemos una única instantánea actual, no un histórico.
- **La atribución de lenguaje sigue la detección de GitHub**, que decide por bytes de archivo y puede etiquetar un proyecto de forma inesperada.
- Otros **4.123** repositorios no tienen fecha de push utilizable y quedan fuera de todos los grupos anteriores.

Las tablas se actualizan con nuestro rastreo diario. Puedes ordenar el mismo corpus por tu cuenta en el [ranking](/).
