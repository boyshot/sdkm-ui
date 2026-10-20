# SDKMAN UI

Interface web local para gerenciar instalações do [SDKMAN](https://sdkman.io/) — lista, instala, remove e define versões padrão de candidates (Java, Maven, Gradle, Kotlin e outros) diretamente pelo browser, com acompanhamento em tempo real do progresso de cada operação.

Pode ser executado como **binário único autocontido** ou via **Docker/Docker Compose**, sem banco de dados ou dependências externas em tempo de execução.

---

## Funcionalidades

- Listar candidates instalados com a versão ativa de cada um
- Listar versões disponíveis na API do SDKMAN mescladas com as instaladas localmente
- Filtrar versões de Java pelo identificador do provedor (por exemplo, `amzn` para Amazon)
- Instalar uma versão com log de progresso em tempo real (SSE)
- Remover uma versão instalada
- Definir a versão padrão (`sdk default`)
- Interface responsiva com tema claro via [Pico.css](https://picocss.com/)

---

## Pré-requisitos

### Execução local (binário)

| Requisito | Versão mínima | Observação |
|---|---|---|
| [Go](https://go.dev/dl/) | 1.22 | Apenas para compilar |
| [SDKMAN](https://sdkman.io/install) | qualquer | Deve estar em `~/.sdkman` |
| Sistema operacional | Linux / macOS | Windows não suportado |

> O binário compilado não requer Go instalado para rodar.

### Execução via Docker

| Requisito | Versão mínima |
|---|---|
| [Docker](https://docs.docker.com/get-docker/) | 24+ |
| [Docker Compose](https://docs.docker.com/compose/) | v2 (plugin) |
| SDKMAN instalado no host | qualquer |

---

## Instalação e execução

### Opção 1 — Binário local

```bash
git clone https://github.com/paulo-rocha/sdkman-ui
cd sdkman-ui
make build
make run
```

`make run` carrega `config.yaml`. Para usar outro arquivo ou sobrescrever o
endereço definido nele:

```bash
make run CONFIG=dev.yaml
make run ADDR=127.0.0.1:9090
```

Acesse o endereço definido em `config.yaml` (por padrão, `127.0.0.1:9000`).

---

### Opção 2 — Docker (container direto)

Constrói a imagem e executa um container montando o `~/.sdkman` do host:

```bash
make docker-run
```

Para usar uma porta diferente:

```bash
make docker-run PORT=9090
```

Para apontar para um diretório SDKMAN alternativo:

```bash
make docker-run SDKMAN_DIR=/opt/sdkman PORT=8080
```

Os comandos Docker também montam `config.yaml`; para selecionar outro arquivo,
use `CONFIG=dev.yaml`.

---

### Opção 3 — Docker Compose (recomendado para uso contínuo)

Sobe o stack em background com build automático:

```bash
make docker-up
```

Para selecionar outro arquivo de configuração, use `make docker-up CONFIG=dev.yaml`.

Com porta e diretório customizados:

```bash
PORT=9090 SDKMAN_DIR=/opt/sdkman make docker-up
```

Ou exportando as variáveis de ambiente:

```bash
export PORT=9090
export SDKMAN_DIR=$HOME/.sdkman
make docker-up
```

Acesse [http://127.0.0.1:8080](http://127.0.0.1:8080) (ou a porta configurada).

---

### Comandos Docker disponíveis

```bash
make docker-build       # Construir imagem Docker
make docker-run         # Construir e executar container isolado
make docker-up          # Subir stack via Docker Compose (background)
make docker-down        # Derrubar stack e remover containers
make docker-logs        # Acompanhar logs do container em tempo real
make docker-shell       # Abrir shell bash dentro do container
make docker-ps          # Verificar status dos containers
make docker-clean       # Remover imagem e fazer prune
```

---

### Variáveis de ambiente do Docker Compose

| Variável | Padrão | Descrição |
|---|---|---|
| `PORT` | `8080` | Porta mapeada no host |
| `SDKMAN_DIR` | `~/.sdkman` | Diretório SDKMAN do host montado no container |
| `IMAGE_NAME` | `sdkman-ui` | Nome da imagem Docker |
| `IMAGE_TAG` | `latest` | Tag da imagem Docker |

---

## Configuração

O SDKMAN UI suporta configuração via **arquivo YAML** e/ou **flags de linha de comando**.

### Arquivo `config.yaml`

```yaml
server:
  host: 127.0.0.1   # IP de escuta (use 0.0.0.0 no Docker)
  port: 9000         # porta TCP

sdkman:
  dir: ~/.sdkman     # diretório base da instalação do SDKMAN

jobs:
  timeout: 10m       # timeout máximo por operação (s, m ou h)
  queue_size: 64     # máximo de operações aguardando na fila
```

Todos os campos são opcionais.

### Flags de linha de comando

```bash
./sdkman-ui -config /etc/sdkman-ui/config.yaml   # arquivo alternativo
./sdkman-ui -addr 127.0.0.1:9090                 # sobrescreve host e porta
./sdkman-ui -help                                 # listar todas as flags
```

### Precedência

```
valores padrão  <  config.yaml  <  flag -addr
```

---

## Comandos Go disponíveis

```bash
make build                  # compilar o binário
make run                    # compilar e executar usando config.yaml
make run CONFIG=dev.yaml    # usar arquivo YAML alternativo
make run ADDR=:9090         # executar em porta alternativa
make test                   # executar testes unitários
make lint                   # go vet (+ golangci-lint se instalado)
make vet                    # go vet
make tidy                   # sincronizar go.mod / go.sum
make clean                  # remover binário gerado
make ci                     # vet + test + build (pipeline completa)
```

---

## API

Todos os endpoints aceitam `HX-Request: true` para retornar HTML parcial compatível com HTMX, ou retornam JSON por padrão.

| Método | Rota | Descrição |
|---|---|---|
| `GET` | `/` | Página principal |
| `GET` | `/candidates` | Lista candidates instalados |
| `GET` | `/candidates/{name}/versions` | Versões disponíveis de um candidate |
| `POST` | `/candidates/{name}/install` | Inicia instalação de uma versão |
| `DELETE` | `/candidates/{name}/versions/{version}` | Remove uma versão instalada |
| `PUT` | `/candidates/{name}/default` | Define versão padrão |
| `GET` | `/jobs/{id}/stream` | Stream SSE com o log de um job |

### Exemplo — instalar uma versão via curl

```bash
# Iniciar instalação (retorna o ID do job)
curl -s -X POST http://127.0.0.1:8080/candidates/java/install \
  -H "Content-Type: application/json" \
  -d '{"version":"21.0.3-tem"}'
# {"job_id":"1"}

# Acompanhar o log em tempo real
curl -s http://127.0.0.1:8080/jobs/1/stream
# event: log
# data: Downloading: java 21.0.3-tem
# ...
# event: status
# data: done
```

---

## Estrutura do Projeto

```
sdk-ui/
├── cmd/server/main.go          # entrypoint
├── internal/
│   ├── config/                 # carregamento do config.yaml
│   ├── sdkman/                 # integração com o SDKMAN (Client, ShellClient, Mock)
│   ├── jobs/                   # fila de jobs e goroutine worker
│   └── api/                    # handlers HTTP, SSE, middleware
├── web/
│   ├── embed.go                # go:embed
│   ├── static/                 # pico.min.css, htmx.min.js, sse.js
│   └── templates/              # layout.html, index.html, partials
├── doc/
│   ├── business.md             # documentação de negócio com diagramas
│   └── technical.md            # documentação técnica com diagramas
├── .kiro/specs/
│   └── architecture.md         # especificação de arquitetura
├── Dockerfile                  # build multi-stage
├── docker-compose.yml          # stack Docker Compose
├── .dockerignore
├── config.yaml                 # configuração padrão
├── Makefile
└── go.mod
```

Documentação adicional:
- Arquitetura detalhada: [`.kiro/specs/architecture.md`](.kiro/specs/architecture.md)
- Negócio e fluxos: [`doc/business.md`](doc/business.md)
- Técnico e diagramas: [`doc/technical.md`](doc/technical.md)

---

## Desenvolvimento

### Executar testes

```bash
make test
# ou
go test -v -race ./...
```

### Adicionar um novo candidate ou ação

1. Adicionar o método à interface `sdkman.Client` em `internal/sdkman/client.go`
2. Implementar em `internal/sdkman/shell.go`
3. Atualizar o `MockClient` em `internal/sdkman/mock.go`
4. Adicionar handler em `internal/api/handlers.go` e registrar em `routes.go`
5. Atualizar o template relevante em `web/templates/`

---

## Segurança

- O servidor escuta exclusivamente em `127.0.0.1` por padrão — não expõe serviço na rede local.
- No Docker, o bind é `0.0.0.0:8080` dentro do container, mas a porta só é acessível pelo host via mapeamento explícito.
- Parâmetros de path (`candidate`, `version`) são validados contra a regex `^[a-z0-9._-]+$` antes de qualquer operação.
- Nenhuma entrada do usuário é interpolada em strings de shell; tudo é passado via argumentos posicionais do bash.
- O container executa com usuário não-root (`appuser`).

---

## Licença

MIT
