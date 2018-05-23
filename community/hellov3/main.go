package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

func sayHello(w http.ResponseWriter, r *http.Request) {
	message := r.URL.Path
	message = strings.TrimPrefix(message, "/")
	message = fmt.Sprintf("<h2><font color=\"#22b176\"> Hi %s. You are awesome</font></h2>", message)
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, message)
}
func main() {
	port := os.Getenv("NOMAD_PORT_web")
	http.HandleFunc("/", sayHello)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		panic(err)
	}
}
