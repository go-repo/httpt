package metric

import (
	"time"
)

type DefaultRequestMetric string

const (
	DefaultRequestMetricDNSLookup         DefaultRequestMetric = "dns_lookup"
	DefaultRequestMetricTCPConnection     DefaultRequestMetric = "tcp_connection"
	DefaultRequestMetricTLSHandshake      DefaultRequestMetric = "tls_handshake"
	DefaultRequestMetricWaitingConnection DefaultRequestMetric = "waiting_connection"
	DefaultRequestMetricSending           DefaultRequestMetric = "sending"
	DefaultRequestMetricWaitingServer     DefaultRequestMetric = "waiting_server"
	DefaultRequestMetricReceiving         DefaultRequestMetric = "receiving"
	DefaultRequestMetricRequestsNumber    DefaultRequestMetric = "requests_number"
)

const DefaultMetricError = "error"

type Metric struct {
	Name      string
	Tags      map[string]string
	Fields    map[string]interface{}
	Timestamp time.Time
}

func NewTypeValueMetric(name string, metricType string, metricValue interface{}, timestamp time.Time) *Metric {
	return &Metric{
		Name:      name,
		Tags:      TypeTag(metricType),
		Fields:    ValueField(metricValue),
		Timestamp: timestamp,
	}
}

type Collector interface {
	// When cancelC is closed, collector needs to start closing.
	// After cancelC is closed, metricC is subsequently closed,
	// collector needs to close doneC after completing the closing work.
	Start(cancelC <-chan struct{}, metricC <-chan *Metric) (doneC <-chan struct{})
}

func TypeTag(metricType string) map[string]string {
	return map[string]string{
		"type": metricType,
	}
}

func ValueField(metricValue interface{}) map[string]interface{} {
	return map[string]interface{}{
		"value": metricValue,
	}
}
