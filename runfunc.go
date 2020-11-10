package httpt

import (
	"net/http"
	"time"

	"github.com/go-repo/httpt/metric"
	"github.com/go-repo/httpt/runfunc"
)

type runFunc struct {
	client  *clientWithTracer
	id      int
	iter    int
	metricC chan<- []*metric.Point
}

// TODO: Add cancel for request.
func (h *runFunc) Request(r *http.Request, options *runfunc.RequestOptions) (runfunc.Response, error) {
	res, err := h.client.Do(r)
	if err != nil {
		return nil, err
	}

	typeVal := ""
	if options != nil && options.Type != "" {
		typeVal = options.Type
	}
	if typeVal == "" {
		typeVal = r.URL.String()
	}

	now := time.Now()
	for _, s := range res.stats {
		h.metricC <- []*metric.Point{
			metric.NewPoint(metric.MeasurementDNSLookup, typeVal, int64(s.DNSLookup), &now),
			metric.NewPoint(metric.MeasurementTCPConnection, typeVal, int64(s.TCPConnection), &now),
			metric.NewPoint(metric.MeasurementTLSHandshake, typeVal, int64(s.TLSHandshake), &now),
			metric.NewPoint(metric.MeasurementWaitingConnection, typeVal, int64(s.WaitingConnection), &now),
			metric.NewPoint(metric.MeasurementSending, typeVal, int64(s.Sending), &now),
			metric.NewPoint(metric.MeasurementWaitingServer, typeVal, int64(s.WaitingServer), &now),
			metric.NewPoint(metric.MeasurementReceiving, typeVal, int64(s.Receiving), &now),
			metric.NewPoint(metric.MeasurementRequestsNumber, typeVal, 1, &now),
		}
	}

	return res, nil
}

func (h *runFunc) ID() int {
	return h.id
}

func (h *runFunc) Iter() int {
	return h.iter
}

func (h *runFunc) AddError(err error, typeVal string) {
	h.metricC <- []*metric.Point{
		metric.NewPoint(metric.MeasurementError, typeVal, err.Error(), metric.TimeNowPointer()),
	}
}

func (h *runFunc) AddMetricPoint(point *metric.Point) {
	h.metricC <- []*metric.Point{point}
}
