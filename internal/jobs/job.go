package jobs

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// Status representa o estado de um job.
type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusError   Status = "error"
)

// jobCounter é um contador atômico global para geração de IDs únicos.
var jobCounter atomic.Uint64

// Job representa uma operação assíncrona (install, uninstall, default).
type Job struct {
	ID        string
	Candidate string
	Version   string
	Action    string // "install" | "uninstall" | "default"

	status Status
	logs   []string
	err    error

	// logCh distribui as linhas de log para os subscribers (SSE).
	// Fechado quando o job termina.
	logCh chan string

	// done é fechado quando o job termina (status Done ou Error).
	done chan struct{}

	// cancel encerra o contexto do job em andamento.
	cancel context.CancelFunc

	// createdAt para ordenação e diagnóstico.
	createdAt time.Time
}

// newJob cria um novo Job com ID único e status Pending.
func newJob(candidate, version, action string) *Job {
	id := fmt.Sprintf("%d", jobCounter.Add(1))
	return &Job{
		ID:        id,
		Candidate: candidate,
		Version:   version,
		Action:    action,
		status:    StatusPending,
		logCh:     make(chan string, 128),
		done:      make(chan struct{}),
		createdAt: time.Now(),
	}
}

// Status retorna o estado atual do job (thread-safe via leitura atômica
// não é necessária aqui pois só o worker escreve, mas a leitura é OK
// porque o campo é atualizado antes do close(done)).
func (j *Job) Status() Status { return j.status }

// Err retorna o erro do job, se houver.
func (j *Job) Err() error { return j.err }

// Logs retorna uma cópia do slice de logs acumulados.
func (j *Job) Logs() []string {
	cp := make([]string, len(j.logs))
	copy(cp, j.logs)
	return cp
}

// LogCh retorna o channel de linhas de log.
// O channel é fechado quando o job termina.
func (j *Job) LogCh() <-chan string { return j.logCh }

// Done retorna um channel que é fechado quando o job termina.
func (j *Job) Done() <-chan struct{} { return j.done }

// Cancel cancela o job em andamento, se aplicável.
func (j *Job) Cancel() {
	if j.cancel != nil {
		j.cancel()
	}
}

// CreatedAt retorna o timestamp de criação do job.
func (j *Job) CreatedAt() time.Time { return j.createdAt }
