# SDKMAN UI — Planejamento de Implementação

## Visão geral

App web fullstack em Go para gerenciar instalações do SDKMAN via interface gráfica local.
Entrega como binário único com assets embutidos, rodando em `127.0.0.1`.

---

## Fase 1 — Scaffold e infraestrutura base

**Objetivo:** estrutura de projeto, módulo Go, dependências e esqueleto dos pacotes.

### Tarefas

- [ ] **T01** — Inicializar módulo Go (`go mod init`) e criar estrutura de diretórios conforme seção 7 dos requisitos.
- [ ] **T02** — Adicionar dependências: `chi` (router), `templ` ou manter `html/template` puro (decisão a confirmar).
- [ ] **T03** — Criar `cmd/server/main.go` com servidor HTTP básico em `127.0.0.1:8080` usando `ServeMux` ou `chi`.
- [ ] **T04** — Configurar `web/embed.go` com `go:embed` para templates e assets estáticos.
- [ ] **T05** — Baixar e embutir `pico.min.css` e `htmx.min.js` em `web/static/`.

**Critério de aceite:** `go build ./...` sem erros; servidor sobe e responde `200` em `/`.

---

## Fase 2 — Camada `sdkman` (leitura e escrita)

**Objetivo:** implementar a interface `sdkman.Client` e sua implementação shell.

### Tarefas

- [ ] **T06** — Definir interface `sdkman.Client` em `internal/sdkman/client.go`:
  ```go
  type Client interface {
      ListCandidates() ([]Candidate, error)
      ListVersions(candidate string) ([]Version, error)
      Install(ctx context.Context, candidate, version string) (<-chan string, error)
      Uninstall(ctx context.Context, candidate, version string) (<-chan string, error)
      SetDefault(ctx context.Context, candidate, version string) error
  }
  ```
- [ ] **T07** — Implementar leitura de candidates instalados via `os.ReadDir` em `~/.sdkman/candidates/` e resolução do symlink `current` com `os.Readlink`.
- [ ] **T08** — Implementar busca de versões disponíveis via API HTTP do SDKMAN (endpoint a descobrir; provavelmente `https://api.sdkman.io/2/candidates/{candidate}/linux/versions/all`).
- [ ] **T09** — Implementar `install`, `uninstall` e `default` via `exec.CommandContext` com `bash -c "source ~/.sdkman/bin/sdkman-init.sh && sdk <cmd> ..."` usando argumentos posicionais.
- [ ] **T10** — Capturar saída linha a linha com `cmd.StdoutPipe()` + `bufio.Scanner` e publicar em channel; remover códigos ANSI.
- [ ] **T11** — Criar `MockClient` em `internal/sdkman/mock.go` para uso nos testes.
- [ ] **T12** — Validar `candidate` e `version` contra regex segura (ex.: `^[a-z0-9._-]+$`) antes de qualquer exec.

**Critério de aceite:** testes unitários do `ShellClient` passando com SDKMAN instalado localmente; mock compilando sem erros.

---

## Fase 3 — Fila de jobs (`internal/jobs`)

**Objetivo:** serializar operações de escrita (install/uninstall/default) em goroutine worker única.

### Tarefas

- [ ] **T13** — Definir struct `Job` com `ID` (UUID ou incremento), `Status` (pending/running/done/error), `Logs []string` e channel de saída.
- [ ] **T14** — Implementar `Queue` com channel interno e goroutine worker que processa um job por vez.
- [ ] **T15** — Expor método `Enqueue(job) (string, error)` retornando o ID do job.
- [ ] **T16** — Armazenar jobs em map protegido por `sync.RWMutex` para consulta de status e logs históricos.
- [ ] **T17** — Implementar cancelamento via `context` com timeout configurável.

**Critério de aceite:** testes unitários com mock garantindo serialização e propagação de cancelamento.

---

## Fase 4 — API HTTP e SSE (`internal/api`)

**Objetivo:** implementar todos os endpoints do rascunho de API (seção 5 dos requisitos).

### Tarefas

- [ ] **T18** — Registrar rotas no router (`chi` ou `ServeMux`):
  | Método | Rota |
  |---|---|
  | GET | `/` |
  | GET | `/candidates` |
  | GET | `/candidates/{name}/versions` |
  | POST | `/candidates/{name}/install` |
  | DELETE | `/candidates/{name}/versions/{version}` |
  | PUT | `/candidates/{name}/default` |
  | GET | `/jobs/{id}/stream` |

- [ ] **T19** — Handler `GET /candidates`: ler via `sdkman.Client` e retornar JSON ou HTML parcial (HTMX).
- [ ] **T20** — Handler `GET /candidates/{name}/versions`: buscar versões disponíveis; retornar lista.
- [ ] **T21** — Handler `POST /candidates/{name}/install`: enfileirar job e retornar `{"job_id": "..."}` com `202 Accepted`.
- [ ] **T22** — Handler `DELETE /candidates/{name}/versions/{version}`: enfileirar job de remoção.
- [ ] **T23** — Handler `PUT /candidates/{name}/default`: enfileirar job de troca de padrão.
- [ ] **T24** — Handler `GET /jobs/{id}/stream`: implementar SSE — escrever `Content-Type: text/event-stream`, flush por linha, fechar quando job terminar.
- [ ] **T25** — Middleware de validação de `candidate` e `version` para todas as rotas que recebem esses parâmetros.

**Critério de aceite:** testes unitários dos handlers com `httptest` e `MockClient`; SSE entregando eventos ao cliente.

---

## Fase 5 — Frontend (templates + HTMX)

**Objetivo:** interface funcional usando Go templates, HTMX e Pico.css.

### Tarefas

- [ ] **T26** — Criar layout base `web/templates/layout.html` com `<head>` incluindo Pico.css e HTMX.
- [ ] **T27** — Página principal (`/`): lista de candidates com versão atual; botão para ver versões disponíveis.
- [ ] **T28** — Componente de lista de versões: tabela com versão, status (instalada/disponível) e ações (instalar/remover/tornar padrão).
- [ ] **T29** — Componente de progresso: área de log atualizada via SSE (`hx-ext="sse"` do HTMX) que exibe as linhas do job em tempo real.
- [ ] **T30** — Feedback visual de estado: botões desabilitados enquanto job está em execução; badge de status do job.

**Critério de aceite:** fluxo completo funcional no browser — listar, instalar, acompanhar log, definir padrão.

---

## Fase 6 — Empacotamento e build

**Objetivo:** binário único, autocontido, pronto para uso.

### Tarefas

- [ ] **T31** — Confirmar que `go:embed` cobre todos os assets (`web/static/`, `web/templates/`).
- [ ] **T32** — Criar `Makefile` com targets: `build`, `run`, `test`, `lint`.
- [ ] **T33** — Testar build cross-platform básico (`GOOS=linux GOARCH=amd64`).
- [ ] **T34** — Documentar uso mínimo no `README.md`: pré-requisitos (SDKMAN instalado), como compilar e rodar.

**Critério de aceite:** `make build` gera binário; executar o binário em máquina com SDKMAN e acessar `http://127.0.0.1:8080`.

---

## Fase 7 — Testes de integração (opcional/futuro)

- [ ] **T35** — Criar container Docker com SDKMAN instalado para testes de integração via Testcontainers.
- [ ] **T36** — Testes de install/uninstall/default contra o container.

---

## Backlog (fora do MVP)

- Histórico persistido de operações.
- Suporte a `sdk upgrade` e detecção de versões desatualizadas.
- Suporte a `sdk use` e arquivos `.sdkmanrc`.
- Tema escuro/claro via Pico.css.

---

## Ordem de execução sugerida

```
Fase 1 → Fase 2 → Fase 3 → Fase 4 → Fase 5 → Fase 6
                                 ↑
                         (paralelo com Fase 5 é possível)
```

## Decisões pendentes

| # | Questão | Opções | Impacto |
|---|---|---|---|
| D1 | Router HTTP | `net/http` ServeMux nativo vs `chi` | baixo |
| D2 | Templates | `html/template` vs `templ` | médio (templ exige codegen) |
| D3 | Frontend alternativo | Templates + HTMX vs React + Vite | alto |
| D4 | Endpoint API SDKMAN | Confirmar URL e formato da resposta | médio |
| D5 | Porta padrão | `8080` vs configurável por flag | baixo |
