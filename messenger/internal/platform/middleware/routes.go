package middleware

import "net/http"

// #region route-errors
// RouteErrors отдаёт ошибки маршрутизации через respond, в формате остального API:
// сам ServeMux пишет 404 и 405 простым текстом. Пустой шаблон из mux.Handler значит
// «маршрута нет» (404) или «путь есть, метода нет» (405 — тогда mux кладёт методы в Allow).
// Найденный маршрут обслуживает mux.ServeHTTP: только так в хендлере заполнен r.Pattern.
func RouteErrors(mux *http.ServeMux, respond func(w http.ResponseWriter, r *http.Request, status int)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, pattern := mux.Handler(r)
		if pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		probe := &headerProbe{header: http.Header{}}
		h.ServeHTTP(probe, r)
		if allow := probe.header.Get("Allow"); allow != "" {
			w.Header().Set("Allow", allow)
		}
		respond(w, r, probe.status)
	})
}

// headerProbe — ResponseWriter-заглушка: забирает у ответа mux статус и Allow, тело выбрасывает.
type headerProbe struct {
	header http.Header
	status int
}

func (p *headerProbe) Header() http.Header { return p.header }

func (p *headerProbe) WriteHeader(code int) {
	if p.status == 0 {
		p.status = code
	}
}

func (p *headerProbe) Write(b []byte) (int, error) {
	if p.status == 0 {
		p.status = http.StatusOK
	}
	return len(b), nil
}

// #endregion route-errors
