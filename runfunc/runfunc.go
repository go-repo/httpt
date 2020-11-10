package runfunc

import (
	"net/http"

	"github.com/go-repo/httpt/metric"
)

type Func func(Param) error

type RequestOptions struct {
	// Used to identify a type of requests,
	// If don't set type, default set to request's URL.
	Type string
}

type Param interface {
	Request(r *http.Request, options *RequestOptions) (Response, error)
	// Start at 0, range is [0, RunnerGroup.Number)
	ID() int
	// Start at 0, each group has its own iteration.
	Iter() int
	// Report a error,
	// typeVal is used to identify a type of errors.
	AddError(err error, typeVal string)
	// Report a metric point.
	AddMetricPoint(*metric.Point)
}

type Response interface {
	// Resp.body is closed.
	Resp() *http.Response

	Body() []byte
}
