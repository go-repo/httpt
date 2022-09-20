package collector

import "github.com/go-repo/httpt/metric"

type CollectMetricFunc func(*metric.Metric)

type DoneFunc func()
