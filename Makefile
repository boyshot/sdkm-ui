# ==============================================================================
# SDKMAN UI — Makefile
# ==============================================================================

BINARY       := sdkman-ui
CMD          := ./cmd/server
MODULE       := $(shell go list -m)
GO           := go
GOFLAGS      := -trimpath
LDFLAGS      := -s -w
CONFIG       ?= config.yaml
ADDR         ?=

# Docker
IMAGE_NAME   ?= sdkman-ui
IMAGE_TAG    ?= latest
COMPOSE_FILE ?= docker-compose.yml
PORT         ?= 8080
SDKMAN_DIR   ?= $(HOME)/.sdkman

# Detectar golangci-lint; usar go vet como fallback
LINTER := $(shell which golangci-lint 2>/dev/null)

.DEFAULT_GOAL := build

# ==============================================================================
# GO — compilação, testes e qualidade
# ==============================================================================

# ------------------------------------------------------------------------------
# build — compila o binário com assets embutidos
# ------------------------------------------------------------------------------
.PHONY: build
build:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)
	@echo "Binário gerado: $(BINARY)"

# ------------------------------------------------------------------------------
# run — compila e executa localmente (use ADDR= para sobrescrever o config)
# ------------------------------------------------------------------------------
.PHONY: run
run: build
	./$(BINARY) -config "$(CONFIG)" $(if $(ADDR),-addr "$(ADDR)")

# ------------------------------------------------------------------------------
# test — executa todos os testes unitários
# ------------------------------------------------------------------------------
.PHONY: test
test:
	$(GO) test -v -race -count=1 ./...

# ------------------------------------------------------------------------------
# lint — golangci-lint se disponível, senão go vet
# ------------------------------------------------------------------------------
.PHONY: lint
lint:
ifdef LINTER
	golangci-lint run ./...
else
	$(GO) vet ./...
	@echo "Dica: instale golangci-lint para análise mais completa."
	@echo "  https://golangci-lint.run/usage/install/"
endif

# ------------------------------------------------------------------------------
# vet — go vet explícito
# ------------------------------------------------------------------------------
.PHONY: vet
vet:
	$(GO) vet ./...

# ------------------------------------------------------------------------------
# tidy — sincronizar go.mod / go.sum
# ------------------------------------------------------------------------------
.PHONY: tidy
tidy:
	$(GO) mod tidy

# ------------------------------------------------------------------------------
# clean — remover binário gerado
# ------------------------------------------------------------------------------
.PHONY: clean
clean:
	@rm -f $(BINARY)
	@echo "Limpo."

# ------------------------------------------------------------------------------
# ci — pipeline completa: vet + test + build
# ------------------------------------------------------------------------------
.PHONY: ci
ci: vet test build

# ==============================================================================
# DOCKER — build, execução e gestão de containers
# ==============================================================================

# ------------------------------------------------------------------------------
# docker-build — constrói a imagem Docker
# ------------------------------------------------------------------------------
.PHONY: docker-build
docker-build:
	docker build \
		--tag $(IMAGE_NAME):$(IMAGE_TAG) \
		--file Dockerfile \
		.
	@echo "Imagem gerada: $(IMAGE_NAME):$(IMAGE_TAG)"

# ------------------------------------------------------------------------------
# docker-run — executa o container diretamente (sem compose)
#   Monta ~/.sdkman do host e mapeia a porta configurada.
#   Uso: make docker-run PORT=9090
# ------------------------------------------------------------------------------
.PHONY: docker-run
docker-run: docker-build
	docker run --rm \
		--name $(IMAGE_NAME) \
		--publish $(PORT):8080 \
		--volume "$(SDKMAN_DIR):/home/appuser/.sdkman" \
		--volume "$(abspath $(CONFIG)):/etc/sdkman-ui/config.yaml:ro" \
		$(IMAGE_NAME):$(IMAGE_TAG) \
		-config /etc/sdkman-ui/config.yaml \
		-addr 0.0.0.0:8080
	@echo "Acesse http://127.0.0.1:$(PORT)"

# ------------------------------------------------------------------------------
# docker-up — sobe o stack via Docker Compose (background)
# ------------------------------------------------------------------------------
.PHONY: docker-up
docker-up:
	PORT=$(PORT) SDKMAN_DIR=$(SDKMAN_DIR) CONFIG_FILE=$(abspath $(CONFIG)) \
	docker compose -f $(COMPOSE_FILE) up --build --detach
	@echo "Acesse http://127.0.0.1:$(PORT)"

# ------------------------------------------------------------------------------
# docker-down — derruba o stack e remove containers
# ------------------------------------------------------------------------------
.PHONY: docker-down
docker-down:
	docker compose -f $(COMPOSE_FILE) down

# ------------------------------------------------------------------------------
# docker-logs — exibe logs do container em tempo real
# ------------------------------------------------------------------------------
.PHONY: docker-logs
docker-logs:
	docker compose -f $(COMPOSE_FILE) logs --follow sdkman-ui

# ------------------------------------------------------------------------------
# docker-shell — abre um shell bash no container em execução
# ------------------------------------------------------------------------------
.PHONY: docker-shell
docker-shell:
	docker compose -f $(COMPOSE_FILE) exec sdkman-ui bash

# ------------------------------------------------------------------------------
# docker-ps — exibe status dos containers do stack
# ------------------------------------------------------------------------------
.PHONY: docker-ps
docker-ps:
	docker compose -f $(COMPOSE_FILE) ps

# ------------------------------------------------------------------------------
# docker-clean — remove imagem e volumes não utilizados (cuidado!)
# ------------------------------------------------------------------------------
.PHONY: docker-clean
docker-clean: docker-down
	docker rmi --force $(IMAGE_NAME):$(IMAGE_TAG) 2>/dev/null || true
	docker image prune --force
	@echo "Imagens limpas."

# ==============================================================================
# HELP
# ==============================================================================
.PHONY: help
help:
	@echo ""
	@echo "SDKMAN UI — targets disponíveis"
	@echo ""
	@echo "  ── Go ─────────────────────────────────────────────────────"
	@echo "  make build              Compilar o binário (padrão)"
	@echo "  make run                Compilar e executar usando $(CONFIG)"
	@echo "  make run CONFIG=outro.yaml  Usar arquivo de configuração alternativo"
	@echo "  make run ADDR=:9090     Sobrescrever endereço configurado"
	@echo "  make test               Executar testes unitários"
	@echo "  make lint               Linter (golangci-lint ou go vet)"
	@echo "  make vet                go vet"
	@echo "  make tidy               Sincronizar go.mod/go.sum"
	@echo "  make clean              Remover binário"
	@echo "  make ci                 vet + test + build"
	@echo ""
	@echo "  ── Docker ─────────────────────────────────────────────────"
	@echo "  make docker-build       Construir imagem Docker"
	@echo "  make docker-run         Construir e executar container"
	@echo "  make docker-run PORT=9090             Porta alternativa"
	@echo "  make docker-up          Subir stack via Docker Compose"
	@echo "  make docker-down        Derrubar stack"
	@echo "  make docker-logs        Logs do container em tempo real"
	@echo "  make docker-shell       Shell bash no container"
	@echo "  make docker-ps          Status dos containers"
	@echo "  make docker-clean       Remover imagem e fazer prune"
	@echo ""
	@echo "  Variáveis configuráveis:"
	@echo "    CONFIG=$(CONFIG)      arquivo de configuração YAML"
	@echo "    PORT=$(PORT)         porta mapeada no host"
	@echo "    SDKMAN_DIR=$(SDKMAN_DIR)"
	@echo "    IMAGE_NAME=$(IMAGE_NAME)    IMAGE_TAG=$(IMAGE_TAG)"
	@echo ""
