package api

import (
	"fmt"
	"net/http"
)

// handleJobStream implementa o endpoint SSE GET /jobs/{id}/stream.
// Envia cada linha de log do job como um evento SSE e fecha a conexão
// quando o job termina ou o cliente desconecta.
func (h *handler) handleJobStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	job := h.queue.Get(id)
	if job == nil {
		http.Error(w, "job não encontrado", http.StatusNotFound)
		return
	}

	// Garantir que o ResponseWriter suporta flush
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming não suportado", http.StatusInternalServerError)
		return
	}

	// Cabeçalhos SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // desabilita buffer no nginx
	w.WriteHeader(http.StatusOK)

	// Enviar logs já acumulados antes de começar a escutar o channel
	for _, line := range job.Logs() {
		writeSSEEvent(w, "log", line)
	}
	flusher.Flush()

	// Escutar novas linhas até o job terminar ou o cliente desconectar
	clientGone := r.Context().Done()
	logCh := job.LogCh()
	done := job.Done()

	for {
		select {
		case line, open := <-logCh:
			if !open {
				// Channel fechado: job terminou
				writeSSEEvent(w, "status", string(job.Status()))
				if err := job.Err(); err != nil {
					writeSSEEvent(w, "error", err.Error())
				}
				flusher.Flush()
				return
			}
			writeSSEEvent(w, "log", line)
			flusher.Flush()

		case <-done:
			writeSSEEvent(w, "status", string(job.Status()))
			flusher.Flush()
			return

		case <-clientGone:
			return
		}
	}
}

// writeSSEEvent escreve um evento SSE no formato "event: <name>\ndata: <data>\n\n".
func writeSSEEvent(w http.ResponseWriter, event, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
}
