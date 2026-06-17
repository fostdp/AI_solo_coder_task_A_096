package alarm_mqtt

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	mqttlib "github.com/eclipse/paho.mqtt.golang"

	"tashan-weir-seepage/internal/database"
	"tashan-weir-seepage/internal/message"
	"tashan-weir-seepage/internal/metrics"
	"tashan-weir-seepage/internal/models"
)

type AlarmMQTT struct {
	store         *database.DataStore
	bus           *message.Bus
	mqttClient    mqttlib.Client
	baseTopic     string
	alarmTopic    string
	dataTopic     string

	sensorConfigs map[string]models.SensorConfig
	cooldowns     map[string]time.Time
	cfgMu         sync.RWMutex

	retryQueue    []*retryItem
	retryMu       sync.Mutex
	retryStop     chan struct{}

	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
}

type retryItem struct {
	Alarm      *models.AlarmRecord
	SensorCfg  *models.SensorConfig
	RetryCount int
	NextTry    time.Time
}

func New(store *database.DataStore, bus *message.Bus) *AlarmMQTT {
	ctx, cancel := context.WithCancel(context.Background())
	return &AlarmMQTT{
		store:         store,
		bus:           bus,
		sensorConfigs: make(map[string]models.SensorConfig),
		cooldowns:     make(map[string]time.Time),
		retryStop:     make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (a *AlarmMQTT) ConnectMQTT() error {
	broker := getEnv("MQTT_BROKER", "tcp://localhost:1883")
	clientID := getEnv("MQTT_CLIENT_ID", "tashan-weir-backend")
	username := os.Getenv("MQTT_USERNAME")
	password := os.Getenv("MQTT_PASSWORD")
	a.baseTopic = getEnv("MQTT_BASE_TOPIC", "tashan-weir")
	a.alarmTopic = a.baseTopic + "/alarm"
	a.dataTopic = a.baseTopic + "/data"

	opts := mqttlib.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	if username != "" {
		opts.SetUsername(username)
	}
	if password != "" {
		opts.SetPassword(password)
	}
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(30 * time.Second)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetCleanSession(true)
	opts.OnConnect = func(c mqttlib.Client) {
		log.Printf("[alarm_mqtt] MQTT connected to %s", broker)
	}
	opts.OnConnectionLost = func(c mqttlib.Client, err error) {
		log.Printf("[alarm_mqtt] MQTT connection lost: %v", err)
	}

	client := mqttlib.NewClient(opts)
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT connect failed: %w", token.Error())
	}
	a.mqttClient = client
	return nil
}

func (a *AlarmMQTT) Start() {
	a.wg.Add(2)
	go a.alarmLoop()
	go a.retryLoop()
	log.Println("[alarm_mqtt] started")
}

func (a *AlarmMQTT) Stop() {
	a.cancel()
	close(a.retryStop)
	a.wg.Wait()
	if a.mqttClient != nil && a.mqttClient.IsConnected() {
		a.mqttClient.Disconnect(250)
	}
	log.Println("[alarm_mqtt] stopped")
}

func (a *AlarmMQTT) UpdateSensorConfigs(configs []models.SensorConfig) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	for _, cfg := range configs {
		a.sensorConfigs[cfg.SensorID] = cfg
	}
	log.Printf("[alarm_mqtt] loaded %d sensor configs", len(configs))
}

func (a *AlarmMQTT) GetConfig(sensorID string) (models.SensorConfig, bool) {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	c, ok := a.sensorConfigs[sensorID]
	return c, ok
}

func (a *AlarmMQTT) alarmLoop() {
	defer a.wg.Done()
	for {
		select {
		case <-a.ctx.Done():
			return
		case req, ok := <-a.bus.AlarmRequestCh:
			if !ok {
				return
			}
			a.evaluateAndPush(req.SensorData)
		}
	}
}

func (a *AlarmMQTT) evaluateAndPush(sd models.SensorData) {
	cfg, ok := a.GetConfig(sd.SensorID)
	if !ok {
		return
	}

	if !a.CheckCooldown(sd.SensorID) {
		return
	}

	alarm := a.EvaluateThreshold(sd, cfg)
	if alarm == nil {
		return
	}

	metrics.IncAlarmTriggered(alarm.AlarmLevel)

	mqttTopic := fmt.Sprintf("%s/%s", a.alarmTopic, alarm.AlarmLevel)
	alarm.MQTTTopic = &mqttTopic

	a.persistAndPublish(alarm, &cfg)
}

func (a *AlarmMQTT) persistAndPublish(alarm *models.AlarmRecord, cfg *models.SensorConfig) {
	if a.store != nil {
		ctxDB, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		id, err := a.store.InsertAlarm(ctxDB, alarm)
		cancel()
		if err != nil {
			log.Printf("[alarm_mqtt] db insert failed: %v", err)
		} else {
			alarm.ID = id
		}
	}

	if err := a.publishAlarm(alarm, cfg); err != nil {
		log.Printf("[alarm_mqtt] publish failed, queued retry: %v", err)
		a.pushRetry(alarm, cfg)
	} else if alarm.ID > 0 && a.store != nil {
		ctxDB, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = a.store.UpdateAlarmMQTTPublished(ctxDB, alarm.ID)
		cancel()
	}
}

func (a *AlarmMQTT) publishAlarm(alarm *models.AlarmRecord, cfg *models.SensorConfig) error {
	if a.mqttClient == nil || !a.mqttClient.IsConnected() {
		return fmt.Errorf("mqtt not connected")
	}

	payload := map[string]interface{}{
		"alarm_id":        alarm.ID,
		"alarm_time":      alarm.AlarmTime.Unix(),
		"alarm_level":     alarm.AlarmLevel,
		"alarm_type":      alarm.AlarmType,
		"sensor_id":       alarm.SensorID,
		"sensor_value":    alarm.SensorValue,
		"threshold_value": alarm.ThresholdValue,
		"message":         alarm.AlarmMessage,
		"dam_name":        "它山堰",
		"timestamp":       time.Now().Unix(),
	}
	if cfg != nil {
		payload["sensor_name"] = cfg.SensorName
		payload["unit"] = cfg.Unit
	}

	data := jsonMarshal(payload)
	topic := fmt.Sprintf("%s/%s", a.alarmTopic, alarm.AlarmLevel)
	token := a.mqttClient.Publish(topic, 2, false, data)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout")
	}
	if token.Error() != nil {
		return token.Error()
	}
	metrics.IncMQTTMessagesPublished()

	a.mqttClient.Publish(fmt.Sprintf("%s/all", a.alarmTopic), 2, false, data)
	metrics.IncMQTTMessagesPublished()
	log.Printf("[alarm_mqtt] alarm pushed id=%d level=%s type=%s", alarm.ID, alarm.AlarmLevel, alarm.AlarmType)
	return nil
}

func (a *AlarmMQTT) PublishSensorData(ctx context.Context, dtuID string, data []models.SensorData) {
	if a.mqttClient == nil || !a.mqttClient.IsConnected() || len(data) == 0 {
		return
	}
	payload := map[string]interface{}{
		"dtu_id":     dtuID,
		"timestamp":  time.Now().Unix(),
		"data_count": len(data),
		"data":       data,
	}
	body := jsonMarshal(payload)
	topic := fmt.Sprintf("%s/sensor/%s", a.dataTopic, dtuID)
	a.mqttClient.Publish(topic, 0, false, body)
	metrics.IncMQTTMessagesPublished()
}

func (a *AlarmMQTT) pushRetry(alarm *models.AlarmRecord, cfg *models.SensorConfig) {
	a.retryMu.Lock()
	defer a.retryMu.Unlock()
	a.retryQueue = append(a.retryQueue, &retryItem{
		Alarm:     alarm,
		SensorCfg: cfg,
		RetryCount: 0,
		NextTry:   time.Now().Add(5 * time.Second),
	})
}

func (a *AlarmMQTT) retryLoop() {
	defer a.wg.Done()
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.retryStop:
			return
		case <-t.C:
			a.processRetry()
		}
	}
}

func (a *AlarmMQTT) processRetry() {
	a.retryMu.Lock()
	if len(a.retryQueue) == 0 {
		a.retryMu.Unlock()
		return
	}
	now := time.Now()
	remaining := a.retryQueue[:0]
	for _, it := range a.retryQueue {
		if it.RetryCount >= 30 {
			log.Printf("[alarm_mqtt] alarm id=%d exceeded max retries, dropped", it.Alarm.ID)
			continue
		}
		if now.After(it.NextTry) {
			if err := a.publishAlarm(it.Alarm, it.SensorCfg); err != nil {
				it.RetryCount++
				it.NextTry = now.Add(time.Duration(5*it.RetryCount) * time.Second)
				remaining = append(remaining, it)
			} else if it.Alarm.ID > 0 && a.store != nil {
				ctxDB, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				_ = a.store.UpdateAlarmMQTTPublished(ctxDB, it.Alarm.ID)
				cancel()
			}
		} else {
			remaining = append(remaining, it)
		}
	}
	a.retryQueue = remaining
	a.retryMu.Unlock()
}

func alarmTypeOf(sensorType string) string {
	switch sensorType {
	case "piezometer":
		return "PORE_PRESSURE_EXCEED"
	case "seepage_meter":
		return "SEEPAGE_FLOW_EXCEED"
	case "scour_depth":
		return "SCOUR_DEPTH_EXCEED"
	case "water_level":
		return "WATER_LEVEL_ABNORMAL"
	case "infiltration_line":
		return "INFILTRATION_LINE_ABNORMAL"
	default:
		return "SENSOR_ABNORMAL"
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func jsonMarshal(v interface{}) []byte {
	out, err := jsonMarshalInternal(v)
	if err != nil {
		return []byte("{}")
	}
	return out
}

func jsonMarshalInternal(v interface{}) ([]byte, error) {
	type jsonIface interface{ MarshalJSON() ([]byte, error) }
	if m, ok := v.(jsonIface); ok {
		return m.MarshalJSON()
	}
	return jsonMarshalFallback(v)
}

func jsonMarshalFallback(v interface{}) ([]byte, error) {
	if v == nil {
		return []byte("null"), nil
	}
	switch t := v.(type) {
	case string:
		return []byte(`"` + t + `"`), nil
	case int, int64, int32, float64, float32, bool:
		return []byte(fmt.Sprintf("%v", t)), nil
	case map[string]interface{}:
		out := []byte("{")
		first := true
		for k, val := range t {
			if !first {
				out = append(out, ',')
			}
			first = false
			kb, _ := jsonMarshalInternal(k)
			out = append(out, kb...)
			out = append(out, ':')
			vb, _ := jsonMarshalInternal(val)
			out = append(out, vb...)
		}
		out = append(out, '}')
		return out, nil
	default:
		return []byte(fmt.Sprintf("%v", t)), nil
	}
}

func (a *AlarmMQTT) EvaluateThreshold(sd models.SensorData, cfg models.SensorConfig) *models.AlarmRecord {
	value := sd.SensorValue

	if cfg.DangerThreshold != nil && value >= *cfg.DangerThreshold {
		return &models.AlarmRecord{
			AlarmTime:      sd.Time,
			AlarmLevel:     "DANGER",
			AlarmType:      alarmTypeOf(cfg.SensorType),
			SensorID:       &cfg.SensorID,
			SensorValue:    &value,
			ThresholdValue: cfg.DangerThreshold,
			AlarmMessage: fmt.Sprintf("[危险] %s %.2f %s 超过危险阈值 %.2f %s",
				cfg.SensorName, value, cfg.Unit, *cfg.DangerThreshold, cfg.Unit),
		}
	}

	if cfg.WarningThreshold != nil && value >= *cfg.WarningThreshold {
		return &models.AlarmRecord{
			AlarmTime:      sd.Time,
			AlarmLevel:     "WARNING",
			AlarmType:      alarmTypeOf(cfg.SensorType),
			SensorID:       &cfg.SensorID,
			SensorValue:    &value,
			ThresholdValue: cfg.WarningThreshold,
			AlarmMessage: fmt.Sprintf("[预警] %s %.2f %s 超过预警阈值 %.2f %s",
				cfg.SensorName, value, cfg.Unit, *cfg.WarningThreshold, cfg.Unit),
		}
	}

	return nil
}

func (a *AlarmMQTT) CheckCooldown(sensorID string) bool {
	cdKey := fmt.Sprintf("%s_alarm", sensorID)
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()

	if last, exists := a.cooldowns[cdKey]; exists && time.Since(last) < 10*time.Minute {
		return false
	}
	a.cooldowns[cdKey] = time.Now()
	return true
}
