package main
import (
  "net/http"
  "strings"
  "os"
  "fmt"
)
func sayHello(w http.ResponseWriter, r *http.Request) {
  message := r.URL.Path
  message = strings.TrimPrefix(message, "/")
  message = fmt.Sprintf("<font color=\"#22b176\">Hi %s. You are awesome</font>", message)
  w.Write([]byte(message))
}
func main() {
  port := os.Getenv("NOMAD_PORT_web")
  http.HandleFunc("/", sayHello)
  if err := http.ListenAndServe(fmt.Sprintf(":%s",port), nil); err != nil {
    panic(err)
  }
}
