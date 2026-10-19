# SDKMAN UI — Requisitos

App web fullstack em Go para gerenciar instalações do SDKMAN (Java, Maven, Gradle etc.) por uma interface gráfica, integrando com o SDKMAN CLI instalado na máquina.

## 1. Objetivo

Oferecer uma interface local para listar, instalar, remover e definir versões padrão de candidates do SDKMAN, com acompanhamento em tempo real do progresso das operações.

## 2. Stack proposta

| Camada | Tecnologia |
|---|---|
| Backend | Go (`net/http`, `ServeMux` do Go 1.22+ ou `chi`) |
| Frontend | Templates Go (`html/template` ou `templ`) + HTMX |
| Estilo | Pico.css (embutido no binário, sem CDN) |
| Tempo real | SSE (Server-Sent Events) |
| Distribuição | Binário único, assets embutidos com `go:embed` |

> Alternativa de frontend: React + Vite com `dist/` embutido via `go:embed`.

## 3. Requisitos funcionais

### MVP
- **RF01** — Listar candidates instalados e a versão atual de cada um.
- **RF02** — Listar versões disponíveis de um candidate.
- **RF03** — Instalar uma versão de um candidate.
- **RF04** — Remover uma versão instalada.
- **RF05** — Definir a versão padrão (`sdk default`).
- **RF06** — Exibir log de progresso em tempo real durante as operações (SSE).

### Futuro (opcional)
- Histórico de operações executadas.
- Atualização (`sdk upgrade`) e verificação de versões desatualizadas.
- Suporte a `sdk use` e a arquivos `.sdkmanrc`.

## 4. Integração com o SDKMAN

O SDKMAN é um conjunto de scripts bash, e `sdk` é uma função de shell. A integração segue esta divisão:

- **Leitura (sem CLI):**
  - Instalados e versão atual: ler `~/.sdkman/candidates/<candidate>/` com `os.ReadDir` e `os.Readlink` (o `current` é um symlink).
  - Versões disponíveis: API HTTP do SDKMAN (sem documentação oficial formal; validar o formato da resposta).
- **Escrita (via CLI):** `install`, `uninstall` e `default` executados com `os/exec`:

```go
cmd := exec.CommandContext(ctx, "bash", "-c",
    `source ~/.sdkman/bin/sdkman-init.sh && sdk install java "$0"`, version)
```

- Capturar a saída com `cmd.StdoutPipe()` + `bufio.Scanner` e enviar cada linha por SSE.
- Configurar `sdkman_auto_answer=true` para evitar prompts interativos.
- Remover códigos ANSI da saída antes de exibir.

## 5. API (rascunho)

| Método | Rota | Descrição |
|---|---|---|
| GET | `/` | Página principal |
| GET | `/candidates` | Lista de candidates instalados |
| GET | `/candidates/{name}/versions` | Versões disponíveis |
| POST | `/candidates/{name}/install` | Inicia instalação (retorna job) |
| DELETE | `/candidates/{name}/versions/{version}` | Remove versão |
| PUT | `/candidates/{name}/default` | Define versão padrão |
| GET | `/jobs/{id}/stream` | SSE com o log do job |

## 6. Requisitos não funcionais

- **Segurança**
  - Bind apenas em `127.0.0.1`.
  - Validar `candidate` e `version` contra lista permitida/regex.
  - Nunca concatenar entrada do usuário na string do shell; usar argumentos posicionais.
- **Concorrência** — operações de escrita serializadas em fila única (goroutine worker consumindo um channel).
- **Robustez** — timeouts e cancelamento via `context`; tratamento de falha do processo.
- **Testabilidade** — interface `sdkman.Client` com implementação shell e mock para testes.
- **Portabilidade** — binário único, funcionando offline (CSS/JS embutidos).

## 7. Estrutura sugerida

```
sdkman-ui/
├── cmd/server/main.go
├── internal/
│   ├── sdkman/     # interface Client + implementação shell
│   ├── jobs/       # fila e worker
│   └── api/        # handlers e SSE
├── web/
│   ├── templates/
│   ├── static/     # pico.min.css, htmx.min.js
│   └── embed.go
└── go.mod
```

## 8. Testes

- Unitários: handlers e fila de jobs com `sdkman.Client` mockado.
- Integração: container com SDKMAN instalado (ex.: Testcontainers) para validar `install`/`default`.
