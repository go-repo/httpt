package httpt

import (
	"strconv"
	"testing"
	"time"

	"github.com/go-repo/assert"
	"github.com/go-repo/httpt/metric"
	"github.com/go-repo/httpt/metric/collector"
	"github.com/go-repo/httpt/runfunc"
)

type testMetricCollector struct {
	basicMetricCollector metric.Collector

	metrics  []*metric.Metric
	doneTime time.Time
}

func newTestMetricCollector() *testMetricCollector {
	var testCollector = &testMetricCollector{}

	testCollector.basicMetricCollector = collector.NewBasicMetricCollector(collector.BasicMetricCollectorConfig{
		CollectMetricFunc: func(m *metric.Metric) {
			testCollector.metrics = append(testCollector.metrics, m)
		},
		DoneFunc: func() {
			testCollector.doneTime = time.Now()
		},
	})

	return testCollector
}

func (x *testMetricCollector) Start(cancelC <-chan struct{}, metricC <-chan *metric.Metric) <-chan struct{} {
	return x.basicMetricCollector.Start(cancelC, metricC)
}

// TODO MetricCollector test is insufficient.
func TestRunner_MetricCollector(t *testing.T) {
	var collector1 = newTestMetricCollector()
	var collector2 = newTestMetricCollector()

	var iteration = 10000
	var runner, err = NewRunner(&RunnerConfig{
		Groups: []*RunnerGroup{
			{
				RunFunc: func(p runfunc.Param) error {
					p.AddMetric(metric.NewTypeValueMetric(strconv.Itoa(p.Iter()), "type", "value", time.Now()))
					return nil
				},
				Number:    100,
				Iteration: iteration,
			},
		},
		Metric: RunnerMetric{
			// Test multiple collectors.
			MetricCollectors: []metric.Collector{collector1, collector2},
		},
	})
	assert.NoError(t, err)

	runner.Run()

	// Assert collected metrics number.

	assert.Equal(t, len(collector1.metrics), iteration)
	assert.Equal(t, len(collector2.metrics), iteration)

	// Assert collected all metrics.

	var assertCollectedAllMetric = func(metrics []*metric.Metric) (maxMetricTime time.Time) {
		var collectedMetricNameMap = map[string]bool{}
		for _, m := range metrics {
			collectedMetricNameMap[m.Name] = true

			if m.Timestamp.After(maxMetricTime) {
				maxMetricTime = m.Timestamp
			}
		}
		for i := 0; i < iteration; i++ {
			assert.Equal(t, collectedMetricNameMap[strconv.Itoa(i)], true)
		}

		return maxMetricTime
	}

	var maxMetricTime1 = assertCollectedAllMetric(collector1.metrics)
	var maxMetricTime2 = assertCollectedAllMetric(collector2.metrics)
	assert.Equal(t, maxMetricTime1.Equal(maxMetricTime2), true)

	// Assert done function last call.

	assert.Equal(t, collector1.doneTime.After(maxMetricTime1), true)
	assert.Equal(t, collector2.doneTime.After(maxMetricTime2), true)
}
