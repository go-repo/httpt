package httpt

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/go-repo/httpt/httpstat"
	"github.com/go-repo/httpt/log"
)

type clientWithTracer struct {
	client *http.Client
}

type response struct {
	// res.body is closed.
	res   *http.Response
	body  []byte
	stats []httpstat.Stat
}

func (r *response) Resp() *http.Response {
	return r.res
}

func (r *response) Body() []byte {
	return r.body
}

type transportWithTracer struct {
	transport http.RoundTripper
	stats     []httpstat.Stat
	body      []byte
}

var defaultTransport http.RoundTripper = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:       30 * time.Second,
		KeepAlive:     30 * time.Second,
		FallbackDelay: -1,
	}).DialContext,
	MaxIdleConns:          10000,
	MaxIdleConnsPerHost:   10000,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

func (t *transportWithTracer) RoundTrip(req *http.Request) (*http.Response, error) {
	tracer := &httpstat.Tracer{}

	reqWithTrace := req.WithContext(httptrace.WithClientTrace(
		req.Context(),
		tracer.ClientTrace(),
	))

	resp, err := t.transport.RoundTrip(reqWithTrace)
	if err != nil {
		if resp != nil && resp.Body != nil {
			// RoundTrip must always close the body, including on errors.
			_ = resp.Body.Close()
		}
		return resp, err
	}

	t.body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		if resp != nil && resp.Body != nil {
			// RoundTrip must always close the body, including on errors.
			_ = resp.Body.Close()
		}
		return resp, err
	}
	err = resp.Body.Close()
	// TODO: Return this error?
	if err != nil {
		log.Log.WithError(err).Error("close request's body")
	}

	tracer.Done(time.Now())

	t.stats = append(t.stats, tracer.Stat)

	return resp, nil
}

// client can't be nil.
func newClient(client *http.Client) *clientWithTracer {
	if client.Transport == nil {
		client.Transport = defaultTransport
	}

	return &clientWithTracer{
		client: client,
	}
}

func (c *clientWithTracer) Do(req *http.Request) (*response, error) {
	transportWithTracer := &transportWithTracer{
		transport: c.client.Transport,
	}

	newClient := http.Client{
		Transport:     transportWithTracer,
		CheckRedirect: c.client.CheckRedirect,
		Jar:           c.client.Jar,
		Timeout:       c.client.Timeout,
	}

	res, err := newClient.Do(req)
	if err != nil {
		return nil, err
	}

	return &response{
		res:   res,
		body:  transportWithTracer.body,
		stats: transportWithTracer.stats,
	}, nil
}
