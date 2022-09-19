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

const (
	MeasurementError             = "error"
	MeasurementDNSLookup         = "dns_lookup"
	MeasurementTCPConnection     = "tcp_connection"
	MeasurementTLSHandshake      = "tls_handshake"
	MeasurementWaitingConnection = "waiting_connection"
	MeasurementSending           = "sending"
	MeasurementWaitingServer     = "waiting_server"
	MeasurementReceiving         = "receiving"
	MeasurementRequestsNumber    = "requests_number"
	MeasurementCPUPercent        = "cpu_percent"
)

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

type Point struct {
	Measurement string
	Tags        map[string]string
	Fields      map[string]interface{}
	T           *time.Time
}

func ValueField(val interface{}) map[string]interface{} {
	return map[string]interface{}{
		"value": val,
	}
}

func TypeTag(val string) map[string]string {
	return map[string]string{
		"type": val,
	}
}

func TimeNowPointer() *time.Time {
	t := time.Now()
	return &t
}

func NewPoint(measurement string, typeVal string, val interface{}, t *time.Time) *Point {
	return &Point{
		Measurement: measurement,
		Tags:        TypeTag(typeVal),
		Fields:      ValueField(val),
		T:           t,
	}
}

type Collector interface {
	// This function is called sequentially,
	// the next function will be called only after the current one returns, this function is blocking.
	CollectMetric(*Metric)
	// This function is called only once, for closing work, this function is blocking.
	Done()
}
