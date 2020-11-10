package testutil

import "net/http"

const (
	HelloPath  = "/hello"
	Hello1Path = "/hello1"
	Hello2Path = "/hello2"
)

func helloHandler(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("hello"))
}

func hello1Handler(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("hello1"))
}

func hello2Handler(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("hello2"))
}

func ServeMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc(HelloPath, helloHandler)
	mux.HandleFunc(Hello1Path, hello1Handler)
	mux.HandleFunc(Hello2Path, hello2Handler)

	return mux
}
