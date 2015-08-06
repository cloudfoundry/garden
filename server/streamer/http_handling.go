package streamer

import (
	"io"
	"net/http"
)

type HandlerFunc func(StreamID, io.Writer)

func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := StreamID(r.FormValue(":streamid"))
	w.WriteHeader(http.StatusOK)

	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer conn.Close()
	h(id, conn)
}
