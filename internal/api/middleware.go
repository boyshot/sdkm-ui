package api

import (
	"net/http"

	"github.com/paulo-rocha/sdkman-ui/internal/sdkman"
)

// validateCandidate é um middleware que valida o path value {name} antes
// de repassar para o handler seguinte.
func validateCandidate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if err := sdkman.ValidateCandidate(name); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		next(w, r)
	}
}

// validateVersion é um middleware que valida o path value {version} antes
// de repassar para o handler seguinte.
func validateVersion(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		version := r.PathValue("version")
		if version != "" {
			if err := sdkman.ValidateVersion(version); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		next(w, r)
	}
}

// chain aplica uma lista de middlewares da esquerda para a direita.
func chain(h http.HandlerFunc, middlewares ...func(http.HandlerFunc) http.HandlerFunc) http.HandlerFunc {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
