package httpt

import "github.com/go-repo/httpt/metric"

func copyMetricChan(number int, cancelC <-chan struct{}, metricC <-chan *metric.Metric, metricChanBufferSize int) []<-chan *metric.Metric {
	if number < 1 {
		return nil
	}

	// If there is only one, not copying can reduce some logic.
	if number == 1 {
		return []<-chan *metric.Metric{metricC}
	}

	var copiedMetricChans = make([]chan *metric.Metric, 0, number)
	var returnMetricChans = make([]<-chan *metric.Metric, 0, number)
	for i := 0; i < number; i++ {
		var ch = make(chan *metric.Metric, metricChanBufferSize)
		copiedMetricChans = append(copiedMetricChans, ch)
		returnMetricChans = append(returnMetricChans, ch)
	}

	go func() {
		for {
			select {
			// Reason for checking ok:
			// Refer https://go.dev/ref/spec#Close,
			// After calling close, and after any previously sent values have been received,
			// receive operations will return the zero value for the channel's type without blocking.
			case m, ok := <-metricC:
				if ok {
					for _, c := range copiedMetricChans {
						c <- m
					}
				}
			case <-cancelC:
				// Send all remaining metrics.
				for m := range metricC {
					for _, c := range copiedMetricChans {
						c <- m
					}
				}

				for _, c := range copiedMetricChans {
					close(c)
				}
				return
			}
		}
	}()

	return returnMetricChans
}

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
		doneCs = append(doneCs, c.Start(cancelC, collectorMetricChans[i]))
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
