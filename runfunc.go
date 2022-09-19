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
	metricC chan<- *metric.Metric
}

// TODO: Add cancel for request.
func (h *runFunc) Request(r *http.Request, options *runfunc.RequestOptions) (runfunc.Response, error) {
	res, err := h.client.Do(r)
	if err != nil {
		return nil, err
	}

	metricType := ""
	if options != nil && options.MetricType != "" {
		metricType = options.MetricType
	}
	if metricType == "" {
		metricType = r.URL.String()
	}

	now := time.Now()
	for _, s := range res.stats {
		h.metricC <- metric.NewTypeValueMetric(string(metric.DefaultRequestMetricDNSLookup), metricType, int64(s.DNSLookup), now)
		h.metricC <- metric.NewTypeValueMetric(string(metric.DefaultRequestMetricTCPConnection), metricType, int64(s.TCPConnection), now)
		h.metricC <- metric.NewTypeValueMetric(string(metric.DefaultRequestMetricTLSHandshake), metricType, int64(s.TLSHandshake), now)
		h.metricC <- metric.NewTypeValueMetric(string(metric.DefaultRequestMetricWaitingConnection), metricType, int64(s.WaitingConnection), now)
		h.metricC <- metric.NewTypeValueMetric(string(metric.DefaultRequestMetricSending), metricType, int64(s.Sending), now)
		h.metricC <- metric.NewTypeValueMetric(string(metric.DefaultRequestMetricWaitingServer), metricType, int64(s.WaitingServer), now)
		h.metricC <- metric.NewTypeValueMetric(string(metric.DefaultRequestMetricReceiving), metricType, int64(s.Receiving), now)
		h.metricC <- metric.NewTypeValueMetric(string(metric.DefaultRequestMetricRequestsNumber), metricType, 1, now)
	}

	return res, nil
}

func (h *runFunc) ID() int {
	return h.id
}

func (h *runFunc) Iter() int {
	return h.iter
}

func (h *runFunc) AddError(err error, metricType string) {
	h.metricC <- metric.NewTypeValueMetric(metric.DefaultMetricError, metricType, err.Error(), time.Now())
}

func (h *runFunc) AddMetric(metric *metric.Metric) {
	h.metricC <- metric
}
