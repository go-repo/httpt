package httpstat

import (
	"crypto/tls"
	"net/http/httptrace"
	"time"
)

type Tracer struct {
	getConn              time.Time
	dnsStart             time.Time
	dnsDone              time.Time
	connectStart         time.Time
	connectDone          time.Time
	tlsHandshakeStart    time.Time
	tlsHandshakeDone     time.Time
	gotConn              time.Time
	wroteRequest         time.Time
	gotFirstResponseByte time.Time

	Stat
}

type Stat struct {
	DNSLookup     time.Duration
	TCPConnection time.Duration
	TLSHandshake  time.Duration
	// Waiting to get a network connection.
	WaitingConnection time.Duration
	Sending           time.Duration
	// Waiting for server first byte.
	WaitingServer time.Duration
	Receiving     time.Duration
}

func (t *Tracer) DNSStart(_ httptrace.DNSStartInfo) {
	t.dnsStart = time.Now()
}

func (t *Tracer) DNSDone(_ httptrace.DNSDoneInfo) {
	t.dnsDone = time.Now()
}

func (t *Tracer) ConnectStart(_, _ string) {
	t.connectStart = time.Now()
}

func (t *Tracer) ConnectDone(_, _ string, _ error) {
	t.connectDone = time.Now()
}

func (t *Tracer) GetConn(_ string) {
	t.getConn = time.Now()
}

func (t *Tracer) GotConn(_ httptrace.GotConnInfo) {
	t.gotConn = time.Now()
}

func (t *Tracer) GotFirstResponseByte() {
	t.gotFirstResponseByte = time.Now()
}

func (t *Tracer) TLSHandshakeStart() {
	t.tlsHandshakeStart = time.Now()
}

func (t *Tracer) TLSHandshakeDone(_ tls.ConnectionState, _ error) {
	t.tlsHandshakeDone = time.Now()
}

func (t *Tracer) WroteRequest(_ httptrace.WroteRequestInfo) {
	t.wroteRequest = time.Now()
}

func (t *Tracer) ClientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GetConn:              t.GetConn,
		GotConn:              t.GotConn,
		GotFirstResponseByte: t.GotFirstResponseByte,
		DNSStart:             t.DNSStart,
		DNSDone:              t.DNSDone,
		ConnectStart:         t.ConnectStart,
		ConnectDone:          t.ConnectDone,
		TLSHandshakeStart:    t.TLSHandshakeStart,
		TLSHandshakeDone:     t.TLSHandshakeDone,
		WroteRequest:         t.WroteRequest,
	}
}

func (t *Tracer) Done(now time.Time) {
	t.DNSLookup = t.dnsDone.Sub(t.dnsStart)
	t.TCPConnection = t.connectDone.Sub(t.connectStart)
	// TODO investigate why connectStart is called but connectDone is not called.
	if t.TCPConnection < 0 {
		t.TCPConnection = 0
	}
	t.TLSHandshake = t.tlsHandshakeDone.Sub(t.tlsHandshakeStart)
	t.WaitingConnection = t.gotConn.Sub(t.getConn)
	t.Sending = t.wroteRequest.Sub(t.gotConn)
	t.WaitingServer = t.gotFirstResponseByte.Sub(t.wroteRequest)
	t.Receiving = now.Sub(t.gotFirstResponseByte)
}
