# ADR 0002: Contrato de tokens de sessão

## Estado

Aceite — 4 de setembro de 2026

## Contexto

Os clientes precisam de uma sessão curta para a API e de um refresh token rotativo para recuperar a sessão. O armazenamento local deve rejeitar valores malformados sem impor um formato diferente do backend.

## Decisão

O backend emite tokens opacos com o prefixo `pv1_` e 43 caracteres Base64URL sem padding, totalizando 47 caracteres. O access token expira em 15 minutos; o refresh token em 30 dias e é invalidado após cada rotação. A base de dados guarda apenas o hash SHA-256 dos tokens.

Cada cliente valida prefixo, comprimento mínimo de 47, campos não vazios e expiração antes de guardar o par no Keystore, Keychain ou DPAPI. Nenhum cliente interpreta o conteúdo nem regista tokens.

## Consequências

O formato pode evoluir com um novo prefixo (`pv2_`) sem migração silenciosa. Testes do backend fixam o comprimento/prefixo; testes de armazenamento cobrem o limite de 47 caracteres. Alterações ao formato exigem atualizar os quatro clientes e a matriz de compatibilidade.
