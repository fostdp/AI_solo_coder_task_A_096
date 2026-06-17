package metrics

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status_code"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	SensorDataReceived = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "sensor_data_received_total",
			Help: "Total number of sensor data points received",
		},
	)

	SeepageSimulationRequests = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "seepage_simulation_requests_total",
			Help: "Total number of seepage simulation requests",
		},
	)

	SeepageSimulationDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "seepage_simulation_duration_seconds",
			Help:    "Seepage simulation calculation duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	OptimizationRequests = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "optimization_requests_total",
			Help: "Total number of optimization algorithm requests",
		},
	)

	OptimizationDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "optimization_duration_seconds",
			Help:    "Optimization algorithm duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	AlarmsTriggered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alarms_triggered_total",
			Help: "Total number of alarms triggered by level",
		},
		[]string{"level"},
	)

	MQTTMessagesPublished = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "mqtt_messages_published_total",
			Help: "Total number of MQTT messages published",
		},
	)

	GoroutineCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "go_goroutines",
			Help: "Number of goroutines",
		},
	)

	MemoryAllocBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "go_memory_alloc_bytes",
			Help: "Bytes of allocated heap objects",
		},
	)

	MemorySysBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "go_memory_sys_bytes",
			Help: "Total bytes of memory obtained from the OS",
		},
	)
)

type Collector struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewCollector() *Collector {
	ctx, cancel := context.WithCancel(context.Background())
	return &Collector{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (c *Collector) Start() {
	c.wg.Add(1)
	go c.collectLoop()
}

func (c *Collector) Stop() {
	c.cancel()
	c.wg.Wait()
}

func (c *Collector) collectLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.collectRuntimeMetrics()
		}
	}
}

func (c *Collector) collectRuntimeMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	GoroutineCount.Set(float64(runtime.NumGoroutine()))
	MemoryAllocBytes.Set(float64(m.Alloc))
	MemorySysBytes.Set(float64(m.Sys))
}

func IncSensorDataReceived() {
	SensorDataReceived.Inc()
}

func IncSeepageSimulationRequests() {
	SeepageSimulationRequests.Inc()
}

func ObserveSeepageSimulationDuration(d time.Duration) {
	SeepageSimulationDuration.Observe(d.Seconds())
}

func IncOptimizationRequests() {
	OptimizationRequests.Inc()
}

func ObserveOptimizationDuration(d time.Duration) {
	OptimizationDuration.Observe(d.Seconds())
}

func IncAlarmTriggered(level string) {
	AlarmsTriggered.WithLabelValues(level).Inc()
}

func IncMQTTMessagesPublished() {
	MQTTMessagesPublished.Inc()
}
