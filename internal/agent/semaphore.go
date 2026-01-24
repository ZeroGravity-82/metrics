package agent

// Semaphore структура семафора
type Semaphore struct {
	semaCh chan struct{}
}

// NewSemaphore создает семафор емкостью maxReq
func NewSemaphore(maxReq int) *Semaphore {
	return &Semaphore{
		semaCh: make(chan struct{}, maxReq),
	}
}

// Acquire пытается получить доступ к семафору
func (s *Semaphore) Acquire() {
	s.semaCh <- struct{}{}
}

// Release освобождает семафор
func (s *Semaphore) Release() {
	<-s.semaCh
}
