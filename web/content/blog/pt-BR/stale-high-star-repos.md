---
title: "Um terço dos repositórios mais estrelados do GitHub está parado há dois anos"
description: "20.435 dos repositórios com muitas estrelas que acompanhamos não recebem um push há mais de dois anos. As explicações óbvias — são todos listas curadas, estão simplesmente prontos — explicam menos do que parece."
date: 2026-07-22
tags: [dados, manutenção]
translatedFrom: en
---

Acompanhamos mais de 67 mil repositórios do GitHub com mil estrelas ou mais e registramos seu estado diariamente. Ordená-los por quanto tempo passou desde o último push produz um número incômodo.

```starrank:staleness
```

Cerca de um terço **não recebeu um único push em mais de dois anos**. Menos da metade foi tocada nos últimos três meses.

Esse número convida a duas objeções imediatas. As duas são razoáveis. Nenhuma sobrevive intacta aos dados.

## Objeção 1: "Isso tudo é awesome-list"

A intuição é que listas curadas, coletâneas de livros e repositórios de preparação para entrevistas acumulam estrelas sem nunca precisar de um commit. Inflam o grupo parado sem dizer nada sobre software.

O efeito é real. Repositórios **sem linguagem de programação detectada** são 5,0% do grupo ativamente mantido e 10,8% do grupo frio de dois anos — exatamente o dobro. Somando linguagens de documentação (Markdown, HTML, TeX, Jupyter), o padrão se mantém: 3,4% contra 6,0%.

Mas juntos isso dá cerca de um sexto do grupo frio. **Os outros cinco sextos são repositórios com uma linguagem de programação real** — software de verdade, intocado por dois anos, com milhares de estrelas apontadas para ele.

Estes são os maiores:

```starrank:stale-repos
limit: 10
```

Alguns são de fato material de referência. Outros são software que as pessoas continuam instalando.

## Objeção 2: "Parado não é abandonado — bom software fica pronto"

Esta é mais forte. Uma biblioteca pequena e focada que resolveu bem seu problema não precisa de commits. Agitação não é saúde, e um repositório silencioso pode estar simplesmente **pronto**.

Se essa fosse a história toda, porém, esperaríamos uma assinatura no rastreador de issues. Um projeto abandonado com usuários reais acumula issues que ninguém tria; um projeto realmente pronto não atrai muitas para começar. Então o acúmulo, normalizado pelo tamanho da audiência, deveria parecer bem diferente entre os dois grupos.

Não parece. Veja a última coluna da tabela: **as issues abertas por mil estrelas ficam entre 9 e 11,5 em todos os grupos**. Projetos ativamente mantidos carregam um acúmulo normalizado ligeiramente **maior** que os frios de dois anos.

O resultado nos surpreendeu — construímos essa coluna esperando que separasse os grupos, e ela se recusou.

## O que essa linha plana provavelmente significa

A leitura mais plausível é que o abandono é mútuo. Projetos raramente ficam em silêncio enquanto usuários seguem martelando o rastreador de issues. A atenção vai embora dos dois lados ao mesmo tempo: quem mantém para de publicar, e quem abriria issues já migrou para outra coisa.

É uma história menos dramática que "milhares de projetos negligenciados com usuários furiosos". Também é uma notícia pior para quem usa estrelas do GitHub para escolher dependências. Um repositório pode ser ao mesmo tempo muito estrelado, visivelmente parado e **sem parecer quebrado por fora** — porque quem reclamaria já foi embora sem dizer nada.

Convém não esticar demais. O que vemos é **um único instantâneo atual** da contagem de issues abertas, e o número do GitHub **inclui pull requests**. Não sabemos se issues foram fechadas em massa, se quem mantém desativou o rastreador, nem como o acúmulo se moveu ao longo do tempo. A linha plana é **compatível** com o abandono mútuo; não o prova.

## A versão prática

A contagem de estrelas registra quantas pessoas um dia acharam que o projeto valia um marcador. Não diz nada sobre se alguém ainda o mantém e — pelos dados de issues — nem muito sobre se alguém ainda o usa.

Antes de adotar uma dependência pela contagem de estrelas, **olhe a data do último push**. Está a um clique e discorda das estrelas em cerca de um terço das vezes.

## O que estes dados não podem dizer

- **A amostra são repositórios que já passaram de 1.000 estrelas**, não todo o GitHub. Nada aqui descreve repositórios típicos.
- **`pushedAt` conta pushes para qualquer branch**, incluindo commits automatizados. É um sinal de vida, não uma medida de trabalho significativo.
- **A contagem de issues abertas inclui pull requests**, e temos um único instantâneo atual, não um histórico.
- **A atribuição de linguagem segue a detecção do próprio GitHub**, que decide por bytes de arquivo e pode rotular um projeto de forma inesperada.
- Outros **4.123** repositórios não têm data de push utilizável e ficam fora de todos os grupos acima.

As tabelas se atualizam com nossa coleta diária. Você pode ordenar o mesmo corpus por conta própria no [ranking](/).
