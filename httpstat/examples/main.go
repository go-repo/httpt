package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/go-repo/httpt/httpstat"
)

func main() {
	tracer := &httpstat.Tracer{}

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		panic(err)
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), tracer.ClientTrace()))
	res, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		panic(err)
	}

	_, err = io.Copy(ioutil.Discard, res.Body)
	if err != nil {
		panic(err)
	}
	_ = res.Body.Close()

	tracer.Done(time.Now())

	fmt.Println("DNSLookup          ", tracer.DNSLookup)
	fmt.Println("TCPConnection      ", tracer.TCPConnection)
	fmt.Println("TLSHandshake       ", tracer.TLSHandshake)
	fmt.Println("WaitingConnection  ", tracer.WaitingConnection)
	fmt.Println("Sending            ", tracer.Sending)
	fmt.Println("WaitingServer      ", tracer.WaitingServer)
	fmt.Println("Receiving          ", tracer.Receiving)
}
