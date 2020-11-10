package influxdb

import (
	"time"

	"github.com/go-repo/httpt/log"
	"github.com/go-repo/httpt/metric"
	influxdb "github.com/influxdata/influxdb1-client/v2"
)

type Config struct {
	Addr     string
	Username string
	Password string
	// Default is httpt.
	DB string
	// Default is 5s.
	WriteInterval time.Duration
	// Default is 10000.
	PointsBufferSize int
}

const (
	defaultDBName           = "httpt"
	defaultWriteInterval    = time.Second * 5
	defaultPointsBufferSize = 10000
)

type influx struct {
	config Config
	client influxdb.Client
}

func Run(
	cfg Config,
	cancelC <-chan struct{},
	metricC <-chan []*metric.Point,
) (
	// Only close after cancelC and metricC were closed.
	doneC <-chan struct{},
	err error,
) {
	inf, err := newInflux(cfg)
	if err != nil {
		return nil, err
	}

	doneChan := make(chan struct{})

	go inf.runGoroutine(cancelC, metricC, doneChan)

	return doneChan, nil
}

func newInflux(cfg Config) (*influx, error) {
	if cfg.DB == "" {
		cfg.DB = defaultDBName
	}
	if cfg.WriteInterval <= 0 {
		cfg.WriteInterval = defaultWriteInterval
	}
	if cfg.PointsBufferSize <= 0 {
		cfg.PointsBufferSize = defaultPointsBufferSize
	}

	client, err := influxdb.NewHTTPClient(influxdb.HTTPConfig{
		Addr:     cfg.Addr,
		Username: cfg.Username,
		Password: cfg.Password,
	})
	if err != nil {
		return nil, err
	}

	return &influx{
		config: cfg,
		client: client,
	}, nil
}

func metricsPointsToInfluxDBPoints(ms []*metric.Point) []*influxdb.Point {
	points := make([]*influxdb.Point, len(ms))
	for i, m := range ms {
		var p *influxdb.Point
		var err error
		if m.T != nil {
			p, err = influxdb.NewPoint(m.Measurement, m.Tags, m.Fields, *m.T)
		} else {
			p, err = influxdb.NewPoint(m.Measurement, m.Tags, m.Fields)
		}
		if err != nil {
			log.Log.WithError(err).Error("new influxdb point")
		}
		points[i] = p
	}
	return points
}

func (i *influx) runGoroutine(
	cancelC <-chan struct{},
	metricC <-chan []*metric.Point,
	doneC chan<- struct{},
) {
	var points []*influxdb.Point

	timer := time.NewTimer(i.config.WriteInterval)
	writeInterval := i.config.WriteInterval
	pointsBufferSize := i.config.PointsBufferSize

	for {
		select {
		case <-timer.C:
			i.batchWrite(&points)
			timer.Reset(writeInterval)
		case ms := <-metricC:
			points = append(points, metricsPointsToInfluxDBPoints(ms)...)
			if len(points) >= pointsBufferSize {
				i.batchWrite(&points)
			}
		case <-cancelC:
			for ms := range metricC {
				points = append(points, metricsPointsToInfluxDBPoints(ms)...)
			}
			i.batchWrite(&points)

			doneC <- struct{}{}
			return
		}
	}
}

func (i *influx) batchWrite(points *[]*influxdb.Point) {
	if points == nil || len(*points) == 0 {
		return
	}

	bp, err := influxdb.NewBatchPoints(influxdb.BatchPointsConfig{
		Database: i.config.DB,
	})
	if err != nil {
		log.Log.WithError(err).Error("new influxdb batch points")
		return
	}

	bp.AddPoints(*points)

	err = i.client.Write(bp)
	if err != nil {
		log.Log.WithError(err).Error("influxdb write")
		return
	}

	*points = nil
}
