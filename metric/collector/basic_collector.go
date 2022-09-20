package collector

import "github.com/go-repo/httpt/metric"

type BasicMetricCollectorConfig struct {
	// This function is called sequentially,
	// the next function will be called only after the current one returns, this function is blocking.
	CollectMetricFunc CollectMetricFunc
	// This function is called only once, for closing work, this function is blocking.
	// This function only is called when all CollectMetricFunc calls are done.
	DoneFunc DoneFunc
}

type basicMetricCollector struct {
	Config BasicMetricCollectorConfig
}

func NewBasicMetricCollector(cfg BasicMetricCollectorConfig) metric.Collector {
	return &basicMetricCollector{
		Config: cfg,
	}
}

func (x *basicMetricCollector) Start(cancelC <-chan struct{}, metricC <-chan *metric.Metric) <-chan struct{} {
	var doneC = make(chan struct{})

	go func() {
		for {
			select {
			// Reason for checking ok:
			// Refer https://go.dev/ref/spec#Close,
			// After calling close, and after any previously sent values have been received,
			// receive operations will return the zero value for the channel's type without blocking.
			case m, ok := <-metricC:
				if ok {
					x.Config.CollectMetricFunc(m)
				}
			case <-cancelC:
				// Collect all remaining metrics.
				for m := range metricC {
					x.Config.CollectMetricFunc(m)
				}

				x.Config.DoneFunc()
				close(doneC)
				return
			}
		}
	}()

	return doneC
}
