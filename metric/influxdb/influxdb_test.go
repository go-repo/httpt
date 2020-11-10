package influxdb

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-repo/assert"
	"github.com/go-repo/httpt/metric"
	client "github.com/influxdata/influxdb1-client/v2"
)

func TestNewInflux(t *testing.T) {
	inf, err := newInflux(Config{
		Addr:             "http://localhost:8060",
		Username:         "admin",
		Password:         "admin",
		DB:               "",
		WriteInterval:    0,
		PointsBufferSize: 0,
	})
	assert.NoError(t, err)

	assert.Equal(t,
		Config{
			Addr:             "http://localhost:8060",
			Username:         "admin",
			Password:         "admin",
			DB:               defaultDBName,
			WriteInterval:    defaultWriteInterval,
			PointsBufferSize: defaultPointsBufferSize,
		},
		inf.config,
	)
	assert.NotEqual(t, inf.client, nil)
}

func TestInflux_RunGoroutine__WriteInterval(t *testing.T) {
	mock := &ClientMock{
		WriteFunc: func(bp client.BatchPoints) error {
			return nil
		},
	}
	inf := &influx{
		config: Config{
			DB:               "test_db",
			WriteInterval:    time.Millisecond,
			PointsBufferSize: defaultPointsBufferSize,
		},
		client: mock,
	}

	cancelC := make(chan struct{})
	metricC := make(chan []*metric.Point)
	doneC := make(chan struct{})

	go inf.runGoroutine(cancelC, metricC, doneC)

	now := time.Now()

	metricC <- []*metric.Point{
		{Measurement: metric.MeasurementError, Fields: metric.ValueField("error 1"), T: &now},
		{Measurement: metric.MeasurementDNSLookup, Fields: metric.ValueField(int64(1)), T: &now},
		{Measurement: metric.MeasurementTCPConnection, Fields: metric.ValueField(int64(2)), T: &now},
		{Measurement: metric.MeasurementTLSHandshake, Fields: metric.ValueField(int64(3)), T: &now},
		{Measurement: metric.MeasurementWaitingConnection, Fields: metric.ValueField(int64(4)), T: &now},
		{Measurement: metric.MeasurementSending, Fields: metric.ValueField(int64(5)), T: &now},
		{Measurement: metric.MeasurementWaitingServer, Fields: metric.ValueField(int64(6)), T: &now},
		{Measurement: metric.MeasurementReceiving, Fields: metric.ValueField(int64(7)), T: &now},
	}
	time.Sleep(time.Millisecond * 2)

	metricC <- []*metric.Point{
		{Measurement: metric.MeasurementError, Fields: metric.ValueField("error 2"), T: &now},
	}
	time.Sleep(time.Millisecond * 2)

	close(cancelC)
	close(metricC)

	<-doneC

	assert.Equal(t, 2, len(mock.WriteCalls()))
	assert.Equal(t, "test_db", mock.WriteCalls()[0].Bp.Database())
	assert.Equal(t, 8, len(mock.WriteCalls()[0].Bp.Points()))
	assert.Equal(t, 1, len(mock.WriteCalls()[1].Bp.Points()))

	nowUnix := now.UnixNano()
	assert.Equal(t,
		fmt.Sprintf(`error value="error 1" %v`, nowUnix),
		mock.WriteCalls()[0].Bp.Points()[0].String(),
	)
	assert.Equal(t,
		fmt.Sprintf(`dns_lookup value=1i %v`, nowUnix),
		mock.WriteCalls()[0].Bp.Points()[1].String(),
	)
	assert.Equal(t,
		fmt.Sprintf(`tcp_connection value=2i %v`, nowUnix),
		mock.WriteCalls()[0].Bp.Points()[2].String(),
	)
	assert.Equal(t,
		fmt.Sprintf(`tls_handshake value=3i %v`, nowUnix),
		mock.WriteCalls()[0].Bp.Points()[3].String(),
	)
	assert.Equal(t,
		fmt.Sprintf(`waiting_connection value=4i %v`, nowUnix),
		mock.WriteCalls()[0].Bp.Points()[4].String(),
	)
	assert.Equal(t,
		fmt.Sprintf(`sending value=5i %v`, nowUnix),
		mock.WriteCalls()[0].Bp.Points()[5].String(),
	)
	assert.Equal(t,
		fmt.Sprintf(`waiting_server value=6i %v`, nowUnix),
		mock.WriteCalls()[0].Bp.Points()[6].String(),
	)
	assert.Equal(t,
		fmt.Sprintf(`receiving value=7i %v`, nowUnix),
		mock.WriteCalls()[0].Bp.Points()[7].String(),
	)

	assert.Equal(t,
		fmt.Sprintf(`error value="error 2" %v`, nowUnix),
		mock.WriteCalls()[1].Bp.Points()[0].String(),
	)
}

func TestInflux_RunGoroutine__PointsBufferSize(t *testing.T) {
	mock := &ClientMock{
		WriteFunc: func(bp client.BatchPoints) error {
			return nil
		},
	}
	inf := &influx{
		config: Config{
			DB:               "test_db",
			WriteInterval:    defaultWriteInterval,
			PointsBufferSize: 5,
		},
		client: mock,
	}

	cancelC := make(chan struct{})
	metricC := make(chan []*metric.Point)
	doneC := make(chan struct{})

	go inf.runGoroutine(cancelC, metricC, doneC)

	now := time.Now()
	for i := 0; i < 5*20; i++ {
		metricC <- []*metric.Point{{Measurement: metric.MeasurementError, Fields: metric.ValueField("error"), T: &now}}
	}

	close(cancelC)
	close(metricC)

	<-doneC

	assert.Equal(t, 20, len(mock.WriteCalls()))
}
