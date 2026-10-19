package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/paulo-rocha/sdkman-ui/internal/sdkman"
)

const (
	// defaultJobTimeout é o timeout máximo para execução de um job.
	defaultJobTimeout = 10 * time.Minute

	// queueBuffer é a capacidade do channel interno da fila.
	queueBuffer = 64
)

// WorkFn é a função que executa o trabalho de um job, retornando um channel
// de log e um possível erro de inicialização.
type WorkFn func(ctx context.Context, candidate, version string) (<-chan string, error)

// Queue serializa a execução de jobs: apenas um job roda por vez.
// Os demais ficam aguardando no channel interno.
type Queue struct {
	mu      sync.RWMutex
	jobs    map[string]*Job
	workCh  chan *Job
	timeout time.Duration
	client  sdkman.Client
}

// NewQueue cria uma Queue e inicia o goroutine worker único em background.
func NewQueue(client sdkman.Client) *Queue {
	return NewQueueWithSize(client, queueBuffer)
}

// NewQueueWithSize cria uma Queue com capacidade de fila configurável.
func NewQueueWithSize(client sdkman.Client, size int) *Queue {
	if size < 1 {
		size = queueBuffer
	}
	q := &Queue{
		jobs:    make(map[string]*Job),
		workCh:  make(chan *Job, size),
		timeout: defaultJobTimeout,
		client:  client,
	}
	go q.worker()
	return q
}

// WithTimeout sobrescreve o timeout padrão dos jobs (útil em testes).
func (q *Queue) WithTimeout(d time.Duration) *Queue {
	q.timeout = d
	return q
}

// Enqueue enfileira um job e retorna seu ID.
// Retorna erro se a fila estiver cheia.
func (q *Queue) Enqueue(candidate, version, action string) (string, error) {
	job := newJob(candidate, version, action)

	q.mu.Lock()
	q.jobs[job.ID] = job
	q.mu.Unlock()

	select {
	case q.workCh <- job:
		return job.ID, nil
	default:
		// Fila cheia — marcar como erro e retornar
		job.status = StatusError
		job.err = fmt.Errorf("fila de jobs está cheia")
		close(job.logCh)
		close(job.done)
		return job.ID, job.err
	}
}

// Get retorna o job com o ID fornecido, ou nil se não existir.
func (q *Queue) Get(id string) *Job {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.jobs[id]
}

// worker é o goroutine único que processa os jobs em série.
func (q *Queue) worker() {
	for job := range q.workCh {
		q.run(job)
	}
}

// run executa um job, publicando logs no channel e atualizando o status.
func (q *Queue) run(job *Job) {
	ctx, cancel := context.WithTimeout(context.Background(), q.timeout)
	job.cancel = cancel
	defer cancel()

	job.status = StatusRunning

	// Selecionar a WorkFn de acordo com a ação
	var (
		logCh <-chan string
		err   error
	)

	switch job.Action {
	case "install":
		logCh, err = q.client.Install(ctx, job.Candidate, job.Version)
	case "uninstall":
		logCh, err = q.client.Uninstall(ctx, job.Candidate, job.Version)
	case "default":
		// SetDefault não retorna channel — simular com channel sintético
		logCh, err = q.runSetDefault(ctx, job)
	default:
		err = fmt.Errorf("ação desconhecida: %s", job.Action)
	}

	if err != nil {
		job.status = StatusError
		job.err = err
		close(job.logCh)
		close(job.done)
		return
	}

	// Encaminhar linhas do channel do sdkman para o channel do job
	for line := range logCh {
		job.logs = append(job.logs, line)
		select {
		case job.logCh <- line:
		case <-ctx.Done():
			break
		}
	}

	if ctx.Err() != nil {
		job.status = StatusError
		job.err = fmt.Errorf("job cancelado ou timeout: %w", ctx.Err())
	} else {
		job.status = StatusDone
	}

	close(job.logCh)
	close(job.done)
}

// runSetDefault adapta SetDefault (que não retorna channel) para o modelo de log.
func (q *Queue) runSetDefault(ctx context.Context, job *Job) (<-chan string, error) {
	ch := make(chan string, 4)
	go func() {
		defer close(ch)
		err := q.client.SetDefault(ctx, job.Candidate, job.Version)
		if err != nil {
			select {
			case ch <- fmt.Sprintf("ERRO: %v", err):
			case <-ctx.Done():
			}
			job.err = err
			return
		}
		select {
		case ch <- fmt.Sprintf("Default definido: %s %s", job.Candidate, job.Version):
		case <-ctx.Done():
		}
	}()
	return ch, nil
}
