package httpt

import "github.com/go-repo/httpt/metric"

func initCollectors(
	cancelC <-chan struct{},
	metricC <-chan *metric.Metric,
	metricChanBufferSize int,
	collectors []metric.Collector,
) <-chan struct{} {
	if len(collectors) == 0 {
		var closedMetricCollectorsDoneC = make(chan struct{})
		close(closedMetricCollectorsDoneC)
		return closedMetricCollectorsDoneC
	}

	var collectorMetricChans = copyMetricChan(len(collectors), cancelC, metricC, metricChanBufferSize)
	var doneCs []<-chan struct{}
	for i, c := range collectors {
		doneCs = append(doneCs, collectorStart(cancelC, collectorMetricChans[i], c))
	}

	var metricCollectorsDoneC = make(chan struct{})
	go func() {
		for _, c := range doneCs {
			<-c
		}
		close(metricCollectorsDoneC)
	}()

	return metricCollectorsDoneC
}

func collectorStart(cancelC <-chan struct{}, metricC <-chan *metric.Metric, collector metric.Collector) <-chan struct{} {
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
					collector.CollectMetric(m)
				}
			case <-cancelC:
				// Collect all remaining metrics.
				for m := range metricC {
					collector.CollectMetric(m)
				}
				collector.Done()

				close(doneC)
				return
			}
		}
	}()

	return doneC
}
