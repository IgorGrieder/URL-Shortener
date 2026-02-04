# Mini Encurtador de URLs para Campanhas Locais

**Nível:** básico  
**Tema(s):** APIs REST, persistência simples, redirecionamento HTTP, rate limiting básico  
**Resumo do problema:**  
Uma pequena agência de marketing precisa de um encurtador de URLs para acompanhar cliques de campanhas locais. O produto inicial deve permitir criar links curtos, redirecionar de forma rápida e registrar contagens de cliques por dia. O sistema será usado por poucos clientes (dezenas) e tráfego moderado (centenas a poucos milhares de cliques/dia).

## Requisitos Funcionais

- Permitir **criar** uma URL curta a partir de uma URL longa via API (ex.: `POST /api/links`).
- **Redirecionar** solicitações `GET /{slug}` para a URL longa correspondente, usando HTTP 301/302.
- Registrar **métrica de cliques** por data (ex.: total diário) e expor consulta simples (ex.: `GET /api/links/{slug}/stats?from=YYYY-MM-DD&to=YYYY-MM-DD`).
- Validar **formato** de URL longa (http/https) e rejeitar entradas inválidas.
- Opcional: permitir **expiração** do link (data de validade) e **notas**/rótulos para organização.

## Requisitos Não Funcionais

- **Latência**: redirecionamento abaixo de 100 ms p95 dentro da mesma região.
- **Disponibilidade**: 99,5% mensal é suficiente para o MVP.
- **Consistência**: leitura do redirecionamento deve ser **forte**; métricas podem ser atualizadas de forma **assíncrona** (eventual).
- **Observabilidade**: logs de acesso, métricas de p95/p99 e contador de erros 5xx.
- **Segurança**: validar domínio de destino opcionalmente (lista de bloqueio), limitar criação por **chave de API**.
- **Custos**: preferir componentes gerenciados/básicos de baixo custo (um banco relacional leve ou KV, cache opcional).

## Diagrama Conceitual (Mermaid)

```mermaid
flowchart LR
  subgraph Client
    U[Usuário/Frontend]
  end

  subgraph API
    G[API Gateway/Router]
    S[Serviço Shortener]
  end

  subgraph Data
    DB[(Banco de Dados Links, Slugs, Expiração )]
    Q[[Fila de Eventos Cliques]]
    M[(Armazenamento de Métricas - Agregado Diário)]
  end

  U -- POST /api/links --> G --> S
  S --> DB
  U -- GET /{slug} --> G --> S
  S --> DB
  S -- 302 --> U

  %% Métricas assíncronas
  S -- Enfileira clique --> Q
  W[Worker de Métricas] --> Q
  W --> M
```

## Desafio

Escalar o sistema para:

- 100 TPS
- 1.000 TPS
- 10.000 TPS
- 100.000 TPS

## Implementação (MVP)

- Arquitetura TPS (Transport / Processing / Storage): ver `docs/PLAN.md`.
- Banco: **MongoDB** (collections `links` e `clicks_daily`).
- Endpoints:
  - `POST /api/links`
  - `GET /{slug}`
  - `GET /api/links/{slug}/stats?from=YYYY-MM-DD&to=YYYY-MM-DD`

## Como rodar

1) Suba MongoDB + API (5 instâncias) + Nginx + Kong:
- `docker compose up -d --build`

2) Teste rápido (Kong e Nginx exigem `X-User`):
- Criar link: `curl -X POST http://localhost:8080/api/links -H 'Content-Type: application/json' -H 'X-User: demo' -d '{"url":"https://example.com"}'`
- Redirecionar: abra `http://localhost:8080/<slug>` com header `X-User`

## Modo High TPS (experimental)

Uma variante focada em throughput está em `cmd/api_hightps`:

- Rodar local: `make run-hightps` (ou `go run ./cmd/api_hightps`)
- Otimizações: buffer de cliques com `BulkWrite` no Mongo
- Defaults otimizados: sem logs/metrics/tracing no hot path (toggle via env)

Variáveis úteis (opcionais):
- `CLICK_BUFFER_ENABLED=true`
- `CLICK_BUFFER_FLUSH_INTERVAL=250ms`
- `CLICK_BUFFER_MAX_BATCH_EVENTS=50000`
- `HTTP_ENABLE_LOGGING=false`
- `HTTP_ENABLE_METRICS=false`
- `REDIRECT_FAST=true`
