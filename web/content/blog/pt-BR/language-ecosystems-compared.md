---
title: "Quantidade não é peso: o que 67 mil repositórios dizem sobre ecossistemas de linguagens"
description: "Acompanhamos diariamente 67 mil repositórios do GitHub com muitas estrelas. Separados por linguagem, a que tem mais projetos não é a que tem o projeto típico mais forte."
date: 2026-07-21
tags: [dados, ecossistemas]
translatedFrom: en
---

O jeito habitual de comparar ecossistemas de linguagens é perguntar qual tem mais projetos. É uma pergunta fácil de responder e fraca para decidir: fala de **quantidade**, não de **peso**.

Acompanhamos mais de 67 mil repositórios do GitHub com muitas estrelas e registramos todos os dias como elas se movem. Separar esse corpus por linguagem mostra algo mais útil do que uma contagem.

## Os números brutos

Esta tabela é lida ao vivo do nosso banco a cada carregamento da página — não é uma captura de tela:

```starrank:languages
limit: 10
```

## Três pontos que valem atenção

### 1. O líder em quantidade não lidera em mediana

Python está bem à frente em número de repositórios, cerca de 1,4 vez o segundo colocado. Mas olhe a **mediana** de estrelas, que responde a "se eu pegar um projeto aleatório nessa linguagem, quão popular ele tende a ser": Python fica atrás de Go e de TypeScript.

Isso é um sinal de **abrangência**. Python cobre computação científica, scraping, aprendizado de máquina, automação — quase tudo. O outro lado da abrangência é uma cauda muito longa, com muitos projetos logo acima do limiar de rastreamento.

### 2. Go tem a maior taxa de acerto

Go tem cerca de um terço dos repositórios de Python e, ainda assim, a maior mediana entre todas as linguagens aqui.

A explicação plausível é a **concentração de domínio**. Os projetos populares em Go se agrupam em infraestrutura cloud-native, DevOps e ferramentas de linha de comando — áreas cujos usuários são eles próprios desenvolvedores e, portanto, pessoas que dão estrelas. Boa parte do alcance de Python atinge públicos que nunca abrem o GitHub.

### 3. Total de estrelas e mediana contam histórias diferentes

JavaScript tem um total de estrelas alto e uma mediana comparativamente baixa. O total reflete um grupo de projetos muito grandes e muito antigos — é **história acumulada**. A mediana fraca sugere que uma fatia menor dos projetos recentes consegue se destacar.

TypeScript inverte isso: menos repositórios que JavaScript, mas mediana mais alta, coerente com projetos novos adotando TypeScript desde o primeiro dia.

## O topo da lista

A composição importa tanto quanto as contagens. Estes são os repositórios com mais estrelas que acompanhamos agora:

```starrank:top-repos
limit: 10
```

Repare quantos são **recursos de aprendizado e listas curadas** em vez de software executável. Eles acumulam quantidades enormes de estrelas enquanto dizem pouco sobre o ecossistema de engenharia de uma linguagem — misturá-los com frameworks e runtimes distorce qualquer comparação.

## O que estes dados não podem dizer

Toda análise deveria declarar os próprios limites:

- **A amostra é de projetos já populares**, não de todo o GitHub. As medianas aqui são medianas *entre projetos que já passaram da barreira*, bem acima do número real de toda a plataforma.
- **Estrelas não são qualidade** nem uso. Elas aproximam a visibilidade entre desenvolvedores. Muitas bibliotecas críticas têm bem menos estrelas do que sua importância real.
- **A atribuição de linguagem segue a detecção de linguagem principal do GitHub.** Um framework de frontend pode acabar rotulado de forma inesperada dependendo de como os bytes dos arquivos se distribuem.
- **Os ecossistemas têm idades diferentes.** C e Java acumulam décadas; Rust teve muito menos tempo. Comparar totais diretamente é injusto com os mais novos.

## Conclusão

Se você está escolhendo uma stack, uma pergunta melhor do que "qual linguagem tem mais projetos" é: **quantas opções maduras existem no nicho específico de que preciso**. Contagens globais ajudam surpreendentemente pouco nisso.

A tabela acima se atualiza com nossa coleta diária. Você pode fatiar os mesmos dados por linguagem e categoria no [ranking](/).
