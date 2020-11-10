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
	"github.com/go-repo/httpt/metric/influxdb"
	"github.com/go-repo/httpt/runfunc"
	"github.com/go-repo/tokenbucket"
	"github.com/shirou/gopsutil/v3/cpu"
)

type RunnerConfig struct {
	// Duration to run,
	// 0 mean no limit.
	Duration time.Duration
	Groups   []*RunnerGroup

	Influxdb *influxdb.Config
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

	unitMutex sync.Mutex
}

type Runner struct {
	Config *RunnerConfig
	// Used for initializing groups,
	// all groups are initialized then close PauseC,
	// mean terminate paused status.
	PauseC  chan struct{}
	CancelC chan struct{}
	// Units and monitorCPU write to MetricC,
	MetricC              chan []*metric.Point
	AllGroupsDone        <-chan struct{}
	MetricCollectorDoneC <-chan struct{}
	// monitorCPU use MetricC so must close before MetricC is closed.
	MonitorCPUDoneC <-chan struct{}
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
	metricC chan<- []*metric.Point,
	atomicFn func() (iter int, isCancel bool),
	initDoneWG *sync.WaitGroup,
) {
	httpImpl := &runFunc{
		id:      id,
		metricC: metricC,
	}

	isInitDone := false
	for {
		// Ignore error because cookiejar.New never return a error.
		jar, _ := cookiejar.New(nil)
		client := newClient(&http.Client{
			// For create a new connection each loop.
			Transport: newTransport(),
			// For share cookie within all requests of one runFunc runtime.
			Jar: jar,
		})
		httpImpl.client = client

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
			param.AddError(errors.New(fmt.Sprint(r)), "panic")
		}
	}()

	err := fn(param)
	if err != nil {
		param.AddError(err, "error")
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
	metricC chan<- []*metric.Point,
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
				idx, group.RunFunc, pauseC, cancelC, metricC,
				atomicUnitFn, &unitInitDoneWG,
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

func monitorCPU(metricC chan<- []*metric.Point, cancelC <-chan struct{}) (monitorCPUDoneC <-chan struct{}) {
	doneC := make(chan struct{})
	go func() {
		for {
			select {
			case <-cancelC:
				close(doneC)
				return
			default:
				p, err := cpu.Percent(time.Second, false)
				if err != nil {
					metricC <- []*metric.Point{
						metric.NewPoint(metric.MeasurementError, "error", err.Error(), metric.TimeNowPointer()),
					}
				}
				if len(p) > 0 {
					metricC <- []*metric.Point{
						metric.NewPoint(metric.MeasurementCPUPercent, "0", p[0], metric.TimeNowPointer()),
					}
				}
			}
		}
	}()
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
	metricC := make(chan []*metric.Point, 2*unitsNum)

	monitorCPUDoneC := monitorCPU(metricC, cancelC)

	var groupDoneChans []<-chan struct{}
	for _, g := range cfg.Groups {
		doneC := initGroup(g, pauseC, cancelC, metricC)
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

	var metricCollectorDoneC <-chan struct{}
	if cfg.Influxdb != nil {
		var err error
		metricCollectorDoneC, err = influxdb.Run(*cfg.Influxdb, cancelC, metricC)
		if err != nil {
			return nil, err
		}
	}

	return &Runner{
		Config:               cfg,
		PauseC:               pauseC,
		CancelC:              cancelC,
		MetricC:              metricC,
		AllGroupsDone:        allGroupsDoneC,
		MetricCollectorDoneC: metricCollectorDoneC,
		MonitorCPUDoneC:      monitorCPUDoneC,
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

	// monitorCPU use MetricC so must close before MetricC is closed.
	<-r.MonitorCPUDoneC

	// For read MetricC is not blocked.
	close(r.MetricC)
	// Wait for MetricCollector to complete the rest of the work.
	<-r.MetricCollectorDoneC
}
