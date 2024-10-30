package httpt

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"

	"github.com/go-repo/httpt/metric"
	"github.com/go-repo/httpt/runfunc"
	"github.com/go-repo/tokenbucket"
)

type RunnerConfig struct {
	// Duration to run,
	// 0 mean no limit.
	Duration time.Duration
	Groups   []*RunnerGroup
	Metric   RunnerMetric
}

type RunnerGroup struct {
	// If func return error then will auto call Param.Error().
	RunFunc runfunc.Func
	// Number of `RunFunc` run concurrently.
	Number int
	// 0 mean no limit.
	MaxRPS float64
	// Fixed iterations of `RunFunc` run.
	Iteration int
	// If true, Http client will not be created inside group loop
	DisableHttpClient bool
	// If true, each request of same concurrency will reuse the same http client. For example, if Number is set to 50, it will create 50 http clients and reuse them.
	ReuseHttpClient bool
	// Customize http.Transport for each group.
	CustomTransport *http.Transport
	// mutex for single group.
	unitMutex sync.Mutex
}

type RunnerMetric struct {
	// All are disabled by default, manually enable if required.
	EnableDefaultRequestMetrics map[metric.DefaultRequestMetric]bool
	MetricCollectors            []metric.Collector
}

type Runner struct {
	Config *RunnerConfig
	// Used for initializing groups,
	// all groups are initialized then close PauseC,
	// mean terminate paused status.
	PauseC                chan struct{}
	CancelC               chan struct{}
	MetricC               chan<- *metric.Metric
	AllGroupsDone         <-chan struct{}
	MetricCollectorsDoneC <-chan struct{}
}

func newTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:       30 * time.Second,
			KeepAlive:     30 * time.Second,
			FallbackDelay: -1,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func unitGoroutine(
	id int,
	fn runfunc.Func,
	pauseC <-chan struct{},
	cancelC <-chan struct{},
	enableDefaultRequestMetrics map[metric.DefaultRequestMetric]bool,
	metricC chan<- *metric.Metric,
	atomicFn func() (iter int, isCancel bool),
	initDoneWG *sync.WaitGroup,
	customTransport *http.Transport,
	reuseHttpClient bool,
	useHttpClient bool,
) {
	httpImpl := &runFunc{
		id:                          id,
		enableDefaultRequestMetrics: enableDefaultRequestMetrics,
		metricC:                     metricC,
	}
	transport := customTransport
	if transport == nil {
		transport = newTransport()
	}

	if useHttpClient && reuseHttpClient {
		jar, _ := cookiejar.New(nil)
		httpImpl.client = newClient(&http.Client{
			Transport: transport,
			Jar:       jar,
		})
	}

	isInitDone := false
	for {
		if useHttpClient && !reuseHttpClient {
			jar, _ := cookiejar.New(nil)
			httpImpl.client = newClient(&http.Client{
				Transport: transport,
				Jar:       jar,
			})
		}

		select {
		case <-cancelC:
			return
		default:
			// If reach max RPS then atomicFn() will be blocked,
			// so put this before the atomicFn()
			// in order not to block initialization.
			if !isInitDone {
				isInitDone = true
				initDoneWG.Done()
			}

			iter, isCancel := atomicFn()
			if isCancel {
				return
			}
			httpImpl.iter = iter

			<-pauseC
			runFuncWithRecover(fn, httpImpl)
			httpImpl.iter = httpImpl.iter + 1
		}
	}
}

func runFuncWithRecover(fn runfunc.Func, param runfunc.Param) {
	defer func() {
		r := recover()
		if r != nil {
			param.AddErrorMetric(errors.New(fmt.Sprint(r)), "panic")
		}
	}()

	err := fn(param)
	if err != nil {
		param.AddErrorMetric(err, "error")
	}
}

func atomicUnitFn(group *RunnerGroup, cancelC <-chan struct{}) func() (iter int, isCancel bool) {
	isEnableRPSLimit := false
	var rpsLimiter *tokenbucket.Limiter
	if group.MaxRPS > 0 {
		isEnableRPSLimit = true
		rpsLimiter = tokenbucket.NewLimiter(group.MaxRPS, group.MaxRPS*2, 0)
	}

	isEnableFixedIter := false
	if group.Iteration > 0 {
		isEnableFixedIter = true
	}

	iter := 0

	// Generate a stopped timer, keep this function has one timer,
	// use timer.Reset function to reuse timer.
	timer := time.NewTimer(0)
	timer.Stop()

	return func() (int, bool) {
		group.unitMutex.Lock()
		defer group.unitMutex.Unlock()

		select {
		case <-cancelC:
			return iter, true
		default:
			if isEnableFixedIter {
				// FIXME: unit is returned but maybe error and metric record has not completed yet,
				//   then we lose some records.
				if iter >= group.Iteration {
					return iter, true
				}
			}

			if isEnableRPSLimit {
				b, sleep := rpsLimiter.Allow(time.Now())
				if !b {
					timer.Reset(sleep)
					select {
					case <-cancelC:
						return iter, true
					case <-timer.C:
					}
				}
			}

			currIter := iter

			iter = iter + 1

			return currIter, false
		}
	}
}

func initGroup(
	group *RunnerGroup,
	pauseC <-chan struct{},
	cancelC <-chan struct{},
	enableDefaultRequestMetrics map[metric.DefaultRequestMetric]bool,
	metricC chan<- *metric.Metric,
) (
	groupDoneC <-chan struct{},
) {
	var unitNum int
	if group.Iteration > 0 && group.Iteration < group.Number {
		unitNum = group.Iteration
	} else {
		unitNum = group.Number
	}

	atomicUnitFn := atomicUnitFn(group, cancelC)
	var unitDoneWG sync.WaitGroup
	var unitInitDoneWG sync.WaitGroup

	for i := 0; i < unitNum; i++ {
		unitDoneWG.Add(1)
		unitInitDoneWG.Add(1)
		go func(idx int) {
			defer unitDoneWG.Done()
			unitGoroutine(
				idx, group.RunFunc, pauseC, cancelC, enableDefaultRequestMetrics,
				metricC, atomicUnitFn, &unitInitDoneWG, group.CustomTransport,
				group.ReuseHttpClient, group.DisableHttpClient,
			)
		}(i)
	}

	doneC := make(chan struct{})
	go func() {
		unitDoneWG.Wait()
		close(doneC)
	}()

	unitInitDoneWG.Wait()

	return doneC
}

func NewRunner(cfg *RunnerConfig) (*Runner, error) {
	pauseC := make(chan struct{})
	cancelC := make(chan struct{})

	var unitsNum int
	for _, g := range cfg.Groups {
		unitsNum = unitsNum + g.Number
	}
	// TODO: Check if 2*unitsNum is reasonable.
	metricChanBufferSize := 2 * unitsNum
	metricC := make(chan *metric.Metric, metricChanBufferSize)

	var groupDoneChans []<-chan struct{}
	for _, g := range cfg.Groups {
		doneC := initGroup(g, pauseC, cancelC, cfg.Metric.EnableDefaultRequestMetrics, metricC)
		groupDoneChans = append(groupDoneChans, doneC)
	}

	allGroupsDoneC := make(chan struct{})
	go func() {
		// Wait for all group are done.
		for _, c := range groupDoneChans {
			<-c
		}
		close(allGroupsDoneC)
	}()

	var metricCollectorsDoneC = initCollectors(cancelC, metricC, metricChanBufferSize, cfg.Metric.MetricCollectors)

	return &Runner{
		Config:                cfg,
		PauseC:                pauseC,
		CancelC:               cancelC,
		MetricC:               metricC,
		AllGroupsDone:         allGroupsDoneC,
		MetricCollectorsDoneC: metricCollectorsDoneC,
	}, nil
}

func (r *Runner) Run() {
	// 3 ways to exit for group:
	// 1. Time to config.duration if set this.
	// 2. Run count to config.Iteration.
	// TODO: 3. (not implement) Exit by the user, for example, press ctrl-c.
	if r.Config.Duration > 0 {
		after := time.After(r.Config.Duration)

		close(r.PauseC)

		select {
		case <-after:
			close(r.CancelC)
			// Ensure all groups are done.
			<-r.AllGroupsDone
		// Maybe run count reach to config.Iteration before time to config.duration.
		case <-r.AllGroupsDone:
			close(r.CancelC)
		}
	} else {
		close(r.PauseC)

		<-r.AllGroupsDone

		close(r.CancelC)
	}

	// The important role of closing the channel is to tell the receiver
	// that the channel is closed and all data must be processed.
	close(r.MetricC)
	// Wait for MetricCollector to complete the rest of the work.
	<-r.MetricCollectorsDoneC
}
