package dtu_receiver

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"tashan-weir-seepage/internal/database"
	"tashan-weir-seepage/internal/message"
	"tashan-weir-seepage/internal/models"
)

const (
	MaxSensorsPerBatch = 256
	MaxSensorIDLen     = 64
)

type DTUReceiver struct {
	store  *database.DataStore
	bus    *message.Bus
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func New(store *database.DataStore, bus *message.Bus) *DTUReceiver {
	ctx, cancel := context.WithCancel(context.Background())
	return &DTUReceiver{
		store:  store,
		bus:    bus,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (r *DTUReceiver) Start() {
	r.wg.Add(1)
	go r.loop()
	log.Println("[dtu_receiver] started")
}

func (r *DTUReceiver) Stop() {
	r.cancel()
	r.wg.Wait()
	log.Println("[dtu_receiver] stopped")
}

func (r *DTUReceiver) loop() {
	defer r.wg.Done()
	for {
		select {
		case <-r.ctx.Done():
			return
		case batch, ok := <-r.bus.SensorBatchCh:
			if !ok {
				return
			}
			r.processSensorBatch(batch)
		}
	}
}

func (r *DTUReceiver) Validate(payload *models.DTUPayload) error {
	if payload.DTUID == "" {
		return errInvalid("dtu_id is required")
	}
	if len(payload.Sensors) == 0 {
		return errInvalid("no sensor data in batch")
	}
	if len(payload.Sensors) > MaxSensorsPerBatch {
		return errInvalid("batch size exceeds limit")
	}

	now := time.Now()
	for i := range payload.Sensors {
		sd := &payload.Sensors[i]
		if sd.SensorID == "" || len(sd.SensorID) > MaxSensorIDLen {
			return errInvalid("sensor_id invalid at index %d", i)
		}
		if math.IsNaN(sd.SensorValue) || math.IsInf(sd.SensorValue, 0) {
			return errInvalid("sensor_value invalid at index %d", i)
		}
		if sd.Time.IsZero() {
			sd.Time = now
		}
		if sd.Time.After(now.Add(5 * time.Minute)) {
			return errInvalid("sensor time in future at index %d", i)
		}
	}
	if payload.Timestamp.IsZero() {
		payload.Timestamp = now
	}
	return nil
}

func (r *DTUReceiver) HandleAndStore(ctx context.Context, payload *models.DTUPayload) (int, error) {
	if err := r.Validate(payload); err != nil {
		return 0, err
	}
	if err := r.store.InsertSensorDataBatch(ctx, payload.Sensors); err != nil {
		return 0, err
	}

	for i := range payload.Sensors {
		select {
		case r.bus.AlarmRequestCh <- message.AlarmRequestMsg{SensorData: payload.Sensors[i]}:
		default:
			log.Printf("[dtu_receiver] alarm channel full, dropping sensor=%s", payload.Sensors[i].SensorID)
		}
	}

	select {
	case r.bus.SensorBatchCh <- message.SensorBatchMsg{
		DTUID:   payload.DTUID,
		Sensors: payload.Sensors,
	}:
	default:
		log.Printf("[dtu_receiver] batch channel full, dropping batch from dtu=%s", payload.DTUID)
	}

	return len(payload.Sensors), nil
}

func (r *DTUReceiver) processSensorBatch(batch message.SensorBatchMsg) {
	log.Printf("[dtu_receiver] received batch: dtu=%s, sensors=%d", batch.DTUID, len(batch.Sensors))
}

type validationError struct {
	msg string
}

func (e *validationError) Error() string { return e.msg }

func errInvalid(format string, args ...interface{}) error {
	return &validationError{msg: "dtu payload: " + fmt.Sprintf(format, args...)}
}
