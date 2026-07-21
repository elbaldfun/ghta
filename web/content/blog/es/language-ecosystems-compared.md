---
title: "Cantidad no es peso: qué dicen 67.000 repositorios sobre los ecosistemas de lenguajes"
description: "Seguimos a diario 67.000 repositorios de GitHub con muchas estrellas. Al separarlos por lenguaje, el que más proyectos tiene no resulta ser el que tiene el proyecto típico más fuerte."
date: 2026-07-21
tags: [datos, ecosistemas]
translatedFrom: en
---

La forma habitual de comparar ecosistemas de lenguajes es preguntar cuál tiene más proyectos. Es una pregunta fácil de responder y débil para actuar: habla de **cantidad**, no de **peso**.

Seguimos más de 67.000 repositorios de GitHub con muchas estrellas y registramos cada día cómo se mueven. Separar ese corpus por lenguaje muestra algo más útil que un recuento.

## Los números en bruto

Esta tabla se lee en vivo desde nuestra base de datos en cada carga de página; no es una captura:

```starrank:languages
limit: 10
```

## Tres cosas que vale la pena notar

### 1. El líder por cantidad no lidera por mediana

Python va muy por delante en número de repositorios, aproximadamente 1,4 veces el segundo. Pero mira la **mediana** de estrellas, que responde a "si elijo un proyecto al azar en este lenguaje, ¿qué popularidad cabe esperar?": Python queda por detrás de Go y de TypeScript.

Eso es una señal de **amplitud**. Python abarca cálculo científico, scraping, aprendizaje automático, automatización: casi todo. La otra cara de la amplitud es una cola muy larga, con muchos proyectos apenas por encima del umbral de seguimiento.

### 2. Go tiene la mayor tasa de acierto

Go tiene alrededor de un tercio de los repositorios de Python y, aun así, la mediana más alta de todos los lenguajes aquí.

La explicación plausible es la **concentración de dominio**. Los proyectos populares de Go se agrupan en infraestructura cloud-native, DevOps y herramientas de línea de comandos: campos cuyos usuarios son a su vez desarrolladores y, por tanto, gente que pone estrellas. Buena parte del alcance de Python llega a públicos que nunca abren GitHub.

### 3. Estrellas totales y mediana cuentan historias distintas

JavaScript tiene un total de estrellas alto y una mediana comparativamente baja. El total refleja un grupo de proyectos muy grandes y muy antiguos: es **historia acumulada**. La mediana floja sugiere que una proporción menor de proyectos recientes logra destacar.

TypeScript lo invierte: menos repositorios que JavaScript pero mediana más alta, coherente con que los proyectos nuevos elijan TypeScript desde el primer día.

## La cabeza de la lista

La composición importa tanto como los recuentos. Estos son los repositorios con más estrellas que seguimos ahora mismo:

```starrank:top-repos
limit: 10
```

Fíjate en cuántos son **recursos de aprendizaje y listas curadas** en lugar de software ejecutable. Acumulan cantidades enormes de estrellas mientras dicen poco sobre el ecosistema de ingeniería de un lenguaje; mezclarlos con frameworks y runtimes distorsiona cualquier comparación.

## Lo que estos datos no pueden decirte

Todo análisis debería declarar sus propios límites:

- **La muestra son proyectos ya populares**, no todo GitHub. Las medianas aquí son medianas *entre proyectos que ya superaron el listón*, muy por encima de la cifra real del sitio completo.
- **Las estrellas no son calidad** ni uso. Aproximan visibilidad entre desarrolladores. Muchas bibliotecas críticas tienen muchas menos estrellas que su importancia real.
- **La atribución de lenguaje sigue la detección de GitHub.** Un framework de frontend puede quedar etiquetado de forma inesperada según cómo se repartan los bytes de sus archivos.
- **Los ecosistemas tienen edades distintas.** C y Java acumulan décadas; Rust ha tenido mucho menos tiempo. Comparar totales directamente es injusto con los más jóvenes.

## Conclusión

Si estás eligiendo tecnología, una pregunta mejor que "qué lenguaje tiene más proyectos" es: **cuántas opciones maduras existen en el nicho concreto que necesito**. Los recuentos globales ayudan sorprendentemente poco con eso.

La tabla de arriba se actualiza sola con nuestro rastreo diario. Puedes segmentar los mismos datos por lenguaje y categoría en el [ranking](/).
