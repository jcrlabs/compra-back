# CLAUDE.md — compra-back

> Dominio prod: `compra-api.jcrlabs.net` | test: `compra-api-test.jcrlabs.net`
> Namespace K8s: `compra`

## Qué es esto

Backend del comparador de precios de supermercados. Ingesta diaria de productos y precios
de Mercadona, Froiz, Gadis, Carrefour, Alcampo y Eroski. API REST para el frontend.

## Stack

- Go 1.25
- Gin · pgx/v5 · JWT
- Scrapers: colly/v2 + goquery (HTML estático), chromedp (JS-heavy futuro)
- Scheduler: robfig/cron/v3 (scraping nocturno 3 AM)

## Estructura

```
cmd/server/main.go          # Entry point
internal/
  config/config.go          # Vars de entorno
  database/database.go      # pgxpool + migrations
  models/                   # Entidades Go
  repository/               # pgx queries
  services/                 # Lógica de negocio
  handlers/                 # HTTP handlers Gin
  middleware/               # Auth, rate limit, logger
  scraper/                  # Scrapers por supermercado
migrations/                 # SQL migrations
deploy/helm/                # Helm chart K8s
.github/workflows/          # CI/CD
```

## Variables de entorno

Ver `.env.example`.

## CI local

```bash
go mod tidy
gofmt -l .
go vet ./...
go test ./... -race
go build -o /dev/null ./cmd/server
```

## Git

- Ramas: `feature/`, `bugfix/`, `hotfix/`, `release/`
- Commits: convencional (`feat:`, `fix:`, `chore:`, etc.)
- Sin mencionar herramientas externas en mensajes de commit

## Health

```
GET /health → { status: "ok", version: "x.y.z" }
GET /livez  → 200 OK
```
