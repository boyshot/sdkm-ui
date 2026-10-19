package api

import (
	"fmt"
	"net/http"

	"github.com/paulo-rocha/sdkman-ui/internal/jobs"
	"github.com/paulo-rocha/sdkman-ui/internal/sdkman"
)

// RegisterRoutes registra todas as rotas da aplicação no mux fornecido.
func RegisterRoutes(mux *http.ServeMux, client sdkman.Client, queue *jobs.Queue) error {
	h, err := newHandler(client, queue)
	if err != nil {
		return fmt.Errorf("inicializar handlers: %w", err)
	}

	// Página principal
	mux.HandleFunc("GET /", h.handleIndex)

	// Candidates
	mux.HandleFunc("GET /candidates",
		h.handleListCandidates)

	mux.HandleFunc("GET /candidates/{name}/versions",
		chain(h.handleListVersions, validateCandidate))

	// Operações de escrita (serializadas via Queue)
	mux.HandleFunc("POST /candidates/{name}/install",
		chain(h.handleInstall, validateCandidate))

	mux.HandleFunc("DELETE /candidates/{name}/versions/{version}",
		chain(h.handleUninstall, validateCandidate, validateVersion))

	mux.HandleFunc("PUT /candidates/{name}/default",
		chain(h.handleSetDefault, validateCandidate))

	// SSE — stream de log de um job
	mux.HandleFunc("GET /jobs/{id}/stream",
		h.handleJobStream)

	return nil
}
