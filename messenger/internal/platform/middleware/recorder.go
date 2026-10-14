package middleware

import "net/http"

// statusRecorder запоминает статус и размер ответа для журнала. Unwrap нужен
// http.ResponseController: через него обработчик доберётся до Flush и таймаутов
// настоящего ResponseWriter, даже если тот обёрнут.
type statusRecorder struct {
	http.ResponseWriter
	status int // 0 — обработчик ничего не записал
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }
