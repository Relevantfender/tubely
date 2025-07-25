package main

import (
	"fmt"
	"net/http"
)

func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		fmt.Printf("Set Cache-Control header for: %s\n", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
