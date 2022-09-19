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
		Name: name,
		Tags: map[string]string{
			"type": metricType,
		},
		Fields: map[string]interface{}{
			"value": metricValue,
		},
		Timestamp: timestamp,
	}
}

type Collector interface {
	// This function is called sequentially,
	// the next function will be called only after the current one returns, this function is blocking.
	CollectMetric(*Metric)
	// This function is called only once, for closing work, this function is blocking.
	Done()
}
