package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type gzipResponseWriter struct {
	http.ResponseWriter
	w *gzip.Writer
}

func (g gzipResponseWriter) Write(b []byte) (int, error) {
	return g.w.Write(b)
}

func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Request decompression
		if strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip") {
			gr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
			defer gr.Close()
			r.Body = struct {
				io.Reader
				io.Closer
			}{Reader: gr, Closer: r.Body}
		}

		// Response compression
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		gw := gzip.NewWriter(w)
		defer gw.Close()
		next.ServeHTTP(gzipResponseWriter{ResponseWriter: w, w: gw}, r)
	})
}
