package httpserver

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
)

// Recover returns middleware that converts a panic into a 500 with the
// stack logged, replacing revel.PanicFilter. onPanic is optional (nil =
// default logger).
func Recover(onPanic func(r *http.Request, panicValue interface{}, stack []byte)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := debug.Stack()
					if onPanic != nil {
						onPanic(r, rec, stack)
					}
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintln(w, "Internal Server Error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// gzipResponseWriter forwards headers/status and pipes the body through a
// gzip stream.
type gzipResponseWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	return g.gz.Write(b)
}

// Gzip returns middleware replicating revel.CompressFilter's observable
// behaviour: when the client sends Accept-Encoding: gzip the response body
// is gzipped and Content-Encoding: gzip + Vary: Accept-Encoding are set.
// Requests without the header pass through untouched.
func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, gz: gz}, r)
	})
}

// LoginRequired is the ported AuthInterceptor for the main-site controllers:
// a commonUrl whitelist marks actions reachable without login; everything
// else requires a non-empty Session["UserId"]. Unauthenticated XHR gets the
// NOTLOGIN JSON envelope, plain requests redirect to /login.
func LoginRequired(whitelist map[string]map[string]bool, anonymous func(c *Context) Result) BeforeFunc {
	return func(c *Context) Result {
		if !needValidateWhitelist(whitelist, c.Controller, c.Action) {
			return nil
		}
		if userId, ok := c.Session["UserId"]; ok && userId != "" {
			return nil
		}
		return anonymous(c)
	}
}

func needValidateWhitelist(whitelist map[string]map[string]bool, controller, method string) bool {
	if actions, ok := whitelist[controller]; ok {
		return !actions[method]
	}
	return true
}
