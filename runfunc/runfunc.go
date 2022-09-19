package runfunc

import (
	"net/http"

	"github.com/go-repo/httpt/metric"
)

type Func func(Param) error

type RequestOptions struct {
	// Used to identify a type of requests,
	// If don't set type, default set to request's URL.
	MetricType string
}

type Param interface {
	Request(r *http.Request, options *RequestOptions) (Response, error)
	// Start at 0, range is [0, RunnerGroup.Number)
	ID() int
	// Start at 0, each group has its own iteration.
	Iter() int
	// Report an error,
	// metricType is used to identify a type of errors.
	AddErrorMetric(err error, metricType string)
	// Report a metric point.
	AddMetric(*metric.Metric)
}

type Response interface {
	// Resp.body is closed.
	Resp() *http.Response

	Body() []byte
}
