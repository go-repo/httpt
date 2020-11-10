package httpstat

import (
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"os"
	"testing"
	"time"

	"github.com/go-repo/assert"
	"github.com/go-repo/httpt/testutil"
)

var (
	server *httptest.Server
)

func buildURL(path string) string {
	return server.URL + path
}

func TestMain(m *testing.M) {
	server = httptest.NewServer(testutil.ServeMux())

	exitCode := m.Run()

	server.Close()

	os.Exit(exitCode)
}

func TestTracer__SendRequest(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, buildURL(testutil.HelloPath), nil)
	assert.NoError(t, err)

	tracer := &Tracer{}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), tracer.ClientTrace()))

	_, err = http.DefaultTransport.RoundTrip(req)
	assert.NoError(t, err)

	tracer.Done(time.Now())
	assert.Equal(t, tracer.DNSLookup, time.Duration(0))
	assert.NotEqual(t, tracer.TCPConnection, time.Duration(0))
	assert.Equal(t, tracer.TLSHandshake, time.Duration(0))
	assert.NotEqual(t, tracer.WaitingConnection, time.Duration(0))
	assert.NotEqual(t, tracer.Sending, time.Duration(0))
	assert.NotEqual(t, tracer.WaitingServer, time.Duration(0))
	assert.NotEqual(t, tracer.Receiving, time.Duration(0))
}

func TestTracer__DoneFunc(t *testing.T) {
	now := time.Now()
	tracer := &Tracer{}
	tracer.getConn = now.Add(1)
	tracer.dnsStart = now.Add(2)
	tracer.dnsDone = now.Add(4)
	tracer.connectStart = now.Add(8)
	tracer.connectDone = now.Add(16)
	tracer.tlsHandshakeStart = now.Add(32)
	tracer.tlsHandshakeDone = now.Add(64)
	tracer.gotConn = now.Add(128)
	tracer.wroteRequest = now.Add(256)
	tracer.gotFirstResponseByte = now.Add(512)
	tracer.Done(now.Add(1024))

	assert.Equal(t, Stat{
		DNSLookup:         4 - 2,
		TCPConnection:     16 - 8,
		TLSHandshake:      64 - 32,
		WaitingConnection: 128 - 1,
		Sending:           256 - 128,
		WaitingServer:     512 - 256,
		Receiving:         1024 - 512,
	}, tracer.Stat)
}
