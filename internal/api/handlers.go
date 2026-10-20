package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"mime"
	"net/http"

	"github.com/paulo-rocha/sdkman-ui/internal/jobs"
	"github.com/paulo-rocha/sdkman-ui/internal/sdkman"
	sdkweb "github.com/paulo-rocha/sdkman-ui/web"
)

// handler agrega as dependências compartilhadas entre todos os handlers.
type handler struct {
	tmpl   *template.Template
	client sdkman.Client
	queue  *jobs.Queue
}

func newHandler(client sdkman.Client, queue *jobs.Queue) (*handler, error) {
	tmpl, err := template.New("").ParseFS(sdkweb.Templates, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("carregar templates: %w", err)
	}
	return &handler{tmpl: tmpl, client: client, queue: queue}, nil
}

// render executa um template nomeado, respondendo 500 em caso de erro.
func (h *handler) render(w http.ResponseWriter, name string, data any) {
	if err := h.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// writeJSON serializa data como JSON com o status informado.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Nada a fazer — headers já enviados
		_ = err
	}
}

// writeError responde com JSON {"error": "..."} e o status informado.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func requestVersion(r *http.Request) (string, error) {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return "", err
	}

	switch contentType {
	case "application/json":
		var body struct {
			Version string `json:"version"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return "", err
		}
		return body.Version, nil
	case "application/x-www-form-urlencoded":
		if err := r.ParseForm(); err != nil {
			return "", err
		}
		return r.PostForm.Get("version"), nil
	default:
		return "", fmt.Errorf("tipo de conteúdo não suportado: %q", contentType)
	}
}

// ----- GET / -----

func (h *handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	candidates, err := h.client.ListCandidates()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "index.html", map[string]any{"Candidates": candidates})
}

// ----- GET /candidates -----

func (h *handler) handleListCandidates(w http.ResponseWriter, r *http.Request) {
	candidates, err := h.client.ListCandidates()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Suporte a HTMX: se o pedido vier com HX-Request, devolver partial HTML
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "candidates-list.html", map[string]any{"Candidates": candidates})
		return
	}
	writeJSON(w, http.StatusOK, candidates)
}

// ----- GET /candidates/{name}/versions -----

func (h *handler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	versions, err := h.client.ListVersions(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if r.Header.Get("HX-Request") == "true" {
		h.render(w, "versions-list.html", map[string]any{
			"Candidate": name,
			"Versions":  versions,
		})
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

// ----- POST /candidates/{name}/install -----

func (h *handler) handleInstall(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.PathValue("version")
	if version == "" {
		var err error
		version, err = requestVersion(r)
		if err != nil || version == "" {
			writeError(w, http.StatusBadRequest, "versão não informada")
			return
		}
	}

	jobID, err := h.queue.Enqueue(name, version, "install")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

// ----- DELETE /candidates/{name}/versions/{version} -----

func (h *handler) handleUninstall(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	version := r.PathValue("version")

	jobID, err := h.queue.Enqueue(name, version, "uninstall")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}

// ----- PUT /candidates/{name}/default -----

func (h *handler) handleSetDefault(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	version, err := requestVersion(r)
	if err != nil || version == "" {
		writeError(w, http.StatusBadRequest, "versão não informada")
		return
	}

	jobID, err := h.queue.Enqueue(name, version, "default")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID})
}
