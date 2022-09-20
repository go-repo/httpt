package collector

import (
	"time"

	"github.com/go-repo/httpt/metric"
)

const (
	defaultMetricsBufferSize      = 10000
	defaultCollectMetricsInterval = 5 * time.Second
)

type BufferMetricCollectorConfig struct {
	// Default is 10000.
	MetricsBufferSize int
	// Default is 5s.
	CollectMetricsInterval time.Duration

	// This function is called sequentially,
	// the next function will be called only after the current one returns, this function is blocking.
	//
	// Call rules:
	// 1. When the number of collected metrics exceeds MetricsBufferSize.
	// 2. When the time reaches CollectMetricsInterval.
	// 3. When receiving a close signal.
	CollectMetricsFunc CollectMetricsFunc
	// This function is called only once, for closing work, this function is blocking.
	// This function only is called when all CollectMetricsFunc calls are done.
	DoneFunc DoneFunc
}

type bufferMetricCollector struct {
	Config BufferMetricCollectorConfig
}

func NewBufferMetricCollector(cfg BufferMetricCollectorConfig) metric.Collector {
	return &bufferMetricCollector{
		Config: cfg,
	}
}

func (x *bufferMetricCollector) Start(cancelC <-chan struct{}, metricC <-chan *metric.Metric) <-chan struct{} {
	var metricsBufferSize int
	if x.Config.MetricsBufferSize > 0 {
		metricsBufferSize = x.Config.MetricsBufferSize
	} else {
		metricsBufferSize = defaultMetricsBufferSize
	}

	var collectMetricsInterval time.Duration
	if x.Config.CollectMetricsInterval > 0 {
		collectMetricsInterval = x.Config.CollectMetricsInterval
	} else {
		collectMetricsInterval = defaultCollectMetricsInterval
	}

	var doneC = make(chan struct{})
	var bufferMetrics []*metric.Metric
	var timer = time.NewTimer(collectMetricsInterval)
	go func() {
		for {
			select {
			case <-timer.C:
				if len(bufferMetrics) == 0 {
					continue
				}

				x.Config.CollectMetricsFunc(bufferMetrics)
				bufferMetrics = nil

				timer.Reset(collectMetricsInterval)

			// Reason for checking ok:
			// Refer https://go.dev/ref/spec#Close,
			// After calling close, and after any previously sent values have been received,
			// receive operations will return the zero value for the channel's type without blocking.
			case m, ok := <-metricC:
				if !ok {
					continue
				}

				bufferMetrics = append(bufferMetrics, m)
				if len(bufferMetrics) > metricsBufferSize {
					x.Config.CollectMetricsFunc(bufferMetrics)
					bufferMetrics = nil
				}

			case <-cancelC:
				for m := range metricC {
					// TODO Check metricsBufferSize?
					bufferMetrics = append(bufferMetrics, m)
				}
				if len(bufferMetrics) > 0 {
					x.Config.CollectMetricsFunc(bufferMetrics)
				}

				x.Config.DoneFunc()
				close(doneC)
				return
			}
		}
	}()

	return doneC
}
