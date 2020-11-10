package metric

import "time"

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

// TODO: Abstract a collector.
type Collector interface {
}
