package worker

// InFlight is how many floors are running now.
func (w *Worker) InFlight() int {
	w.Probes.mu.Lock()
	defer w.Probes.mu.Unlock()
	return len(w.Probes.inflight)
}
