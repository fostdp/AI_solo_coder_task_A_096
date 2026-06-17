package message

import (
	"sync"
	"time"

	"tashan-weir-seepage/internal/models"
)

type MsgType int

const (
	MsgTypeSensorBatch MsgType = iota
	MsgTypeAlarmRequest
	MsgTypeAlarmPublished
	MsgTypeSimRequest
	MsgTypeSimResult
	MsgTypeOptRequest
	MsgTypeOptResult
	MsgTypeModuleStatus
	MsgTypeConfigUpdate
)

type Message struct {
	Type      MsgType
	Data      interface{}
	Timestamp time.Time
}

type SensorBatchMsg struct {
	DTUID     string
	Sensors   []models.SensorData
	Timestamp time.Time
}

type SimRequestMsg struct {
	RequestID        string
	UpstreamWL       float64
	DownstreamWL     float64
	BlanketLength    float64
	BlanketThickness float64
	Permeability     float64
	ResponseCh       chan *SimResultMsg
}

type SimResultMsg struct {
	RequestID        string
	Success          bool
	Error            string
	Simulation       *models.SeepageSimulation
	Grids            []models.SimulationGrid
	SeepageFlow      float64
	MaxPorePressure  float64
}

type OptRequestMsg struct {
	RequestID    string
	UpstreamWL   float64
	DownstreamWL float64
	ResponseCh   chan *OptResultMsg
}

type OptResultMsg struct {
	RequestID   string
	Success     bool
	Error       string
	Result      *models.OptimizationResult
	ParetoFront []models.ParetoSolution
	ElapsedMs   int64
}

type AlarmRequestMsg struct {
	SensorData models.SensorData
}

type AlarmPublishedMsg struct {
	AlarmID int64
	Success bool
	Error   string
}

type ModuleStatusMsg struct {
	ModuleName string
	Status     string
	Timestamp  time.Time
}

type ConfigUpdateMsg struct {
	ConfigType string
	Data       interface{}
}

type Bus struct {
	SensorBatchCh    chan SensorBatchMsg
	SimRequestCh     chan SimRequestMsg
	OptRequestCh     chan OptRequestMsg
	AlarmRequestCh   chan AlarmRequestMsg
	AlarmPublishCh   chan AlarmPublishedMsg
	ModuleStatusCh   chan ModuleStatusMsg
	ConfigUpdateCh   chan ConfigUpdateMsg

	wg       sync.WaitGroup
	closed   bool
	mu       sync.Mutex
}

func NewBus(bufferSize int) *Bus {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	return &Bus{
		SensorBatchCh:  make(chan SensorBatchMsg, bufferSize),
		SimRequestCh:   make(chan SimRequestMsg, bufferSize),
		OptRequestCh:   make(chan OptRequestMsg, bufferSize),
		AlarmRequestCh: make(chan AlarmRequestMsg, bufferSize),
		AlarmPublishCh: make(chan AlarmPublishedMsg, bufferSize),
		ModuleStatusCh: make(chan ModuleStatusMsg, bufferSize),
		ConfigUpdateCh: make(chan ConfigUpdateMsg, bufferSize),
	}
}

func (b *Bus) PublishSensorBatch(msg SensorBatchMsg) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return false
	}
	select {
	case b.SensorBatchCh <- msg:
		return true
	default:
		return false
	}
}

func (b *Bus) PublishAlarmRequest(msg AlarmRequestMsg) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return false
	}
	select {
	case b.AlarmRequestCh <- msg:
		return true
	default:
		return false
	}
}

func (b *Bus) PublishSimRequest(msg SimRequestMsg) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return false
	}
	select {
	case b.SimRequestCh <- msg:
		return true
	default:
		return false
	}
}

func (b *Bus) PublishOptRequest(msg OptRequestMsg) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return false
	}
	select {
	case b.OptRequestCh <- msg:
		return true
	default:
		return false
	}
}

func (b *Bus) PublishModuleStatus(moduleName, status string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	msg := ModuleStatusMsg{
		ModuleName: moduleName,
		Status:     status,
		Timestamp:  time.Now(),
	}
	select {
	case b.ModuleStatusCh <- msg:
	default:
	}
}

func (b *Bus) PublishConfigUpdate(configType string, data interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	msg := ConfigUpdateMsg{
		ConfigType: configType,
		Data:       data,
	}
	select {
	case b.ConfigUpdateCh <- msg:
	default:
	}
}

func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	b.closed = true

	close(b.SensorBatchCh)
	close(b.SimRequestCh)
	close(b.OptRequestCh)
	close(b.AlarmRequestCh)
	close(b.AlarmPublishCh)
	close(b.ModuleStatusCh)
	close(b.ConfigUpdateCh)
}

func (b *Bus) IsClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}
