package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"tashan-weir-seepage/internal/models"
)

type MQTTService struct {
	client      mqtt.Client
	baseTopic   string
	alarmTopic  string
	dataTopic   string
}

type MQTTAlarmMessage struct {
	AlarmID       int64     `json:"alarm_id"`
	AlarmTime     time.Time `json:"alarm_time"`
	AlarmLevel    string    `json:"alarm_level"`
	AlarmType     string    `json:"alarm_type"`
	SensorID      *string   `json:"sensor_id,omitempty"`
	SensorName    string    `json:"sensor_name,omitempty"`
	SensorValue   *float64  `json:"sensor_value,omitempty"`
	Threshold     *float64  `json:"threshold_value,omitempty"`
	Unit          string    `json:"unit,omitempty"`
	Message       string    `json:"message"`
	DamName       string    `json:"dam_name"`
	Location      string    `json:"location"`
	Timestamp     int64     `json:"timestamp"`
}

func NewMQTTService() (*MQTTService, error) {
	broker := getEnv("MQTT_BROKER", "tcp://localhost:1883")
	clientID := getEnv("MQTT_CLIENT_ID", "tashan-weir-backend")
	username := os.Getenv("MQTT_USERNAME")
	password := os.Getenv("MQTT_PASSWORD")
	baseTopic := getEnv("MQTT_BASE_TOPIC", "tashan-weir")

	opts := mqtt.NewClientOptions()
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

	opts.OnConnect = func(c mqtt.Client) {
		log.Printf("MQTT connected to %s", broker)
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
	}
	opts.OnReconnecting = func(c mqtt.Client, opts *mqtt.ClientOptions) {
		log.Printf("MQTT reconnecting...")
	}

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("MQTT connect failed: %w", token.Error())
	}

	return &MQTTService{
		client:     client,
		baseTopic:  baseTopic,
		alarmTopic: baseTopic + "/alarm",
		dataTopic:  baseTopic + "/data",
	}, nil
}

func (m *MQTTService) Close() {
	if m.client != nil && m.client.IsConnected() {
		m.client.Disconnect(250)
		log.Println("MQTT client disconnected")
	}
}

func (m *MQTTService) PublishAlarm(ctx context.Context, alarm *models.AlarmRecord, sensorConfig *models.SensorConfig) error {
	msg := MQTTAlarmMessage{
		AlarmID:     alarm.ID,
		AlarmTime:   alarm.AlarmTime,
		AlarmLevel:  alarm.AlarmLevel,
		AlarmType:   alarm.AlarmType,
		SensorID:    alarm.SensorID,
		SensorValue: alarm.SensorValue,
		Threshold:   alarm.ThresholdValue,
		Message:     alarm.AlarmMessage,
		DamName:     "它山堰",
		Location:    "浙江省宁波市鄞州区",
		Timestamp:   time.Now().Unix(),
	}

	if sensorConfig != nil {
		msg.SensorName = sensorConfig.SensorName
		msg.Unit = sensorConfig.Unit
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal alarm message failed: %w", err)
	}

	topic := fmt.Sprintf("%s/%s", m.alarmTopic, alarm.AlarmLevel)
	qos := byte(2)
	retained := false

	token := m.client.Publish(topic, qos, retained, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("MQTT publish timeout")
	}
	if token.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %w", token.Error())
	}

	broadTopic := fmt.Sprintf("%s/all", m.alarmTopic)
	m.client.Publish(broadTopic, qos, retained, payload)

	log.Printf("Alarm published to MQTT: level=%s, type=%s, id=%d",
		alarm.AlarmLevel, alarm.AlarmType, alarm.ID)

	return nil
}

func (m *MQTTService) PublishSensorData(ctx context.Context, dtuID string, data []models.SensorData) error {
	if len(data) == 0 {
		return nil
	}

	payload := map[string]interface{}{
		"dtu_id":     dtuID,
		"timestamp":  time.Now().Unix(),
		"data_count": len(data),
		"data":       data,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	topic := fmt.Sprintf("%s/sensor/%s", m.dataTopic, dtuID)
	token := m.client.Publish(topic, 0, false, jsonData)
	token.WaitTimeout(3 * time.Second)

	return nil
}

func (m *MQTTService) PublishStatus(ctx context.Context, status map[string]interface{}) error {
	status["timestamp"] = time.Now().Unix()
	jsonData, err := json.Marshal(status)
	if err != nil {
		return err
	}

	topic := fmt.Sprintf("%s/status", m.baseTopic)
	token := m.client.Publish(topic, 1, true, jsonData)
	token.WaitTimeout(3 * time.Second)

	return nil
}

type AlarmChecker struct {
	sensorConfigs map[string]models.SensorConfig
	cooldowns     map[string]time.Time
}

func NewAlarmChecker() *AlarmChecker {
	return &AlarmChecker{
		sensorConfigs: make(map[string]models.SensorConfig),
		cooldowns:     make(map[string]time.Time),
	}
}

func (ac *AlarmChecker) UpdateSensorConfigs(configs []models.SensorConfig) {
	for _, cfg := range configs {
		ac.sensorConfigs[cfg.SensorID] = cfg
	}
}

func (ac *AlarmChecker) CheckSensor(sensorData models.SensorData) *models.AlarmRecord {
	cfg, ok := ac.sensorConfigs[sensorData.SensorID]
	if !ok {
		return nil
	}

	cooldownKey := fmt.Sprintf("%s_%s", sensorData.SensorID, "alarm")
	if last, exists := ac.cooldowns[cooldownKey]; exists {
		if time.Since(last) < 10*time.Minute {
			return nil
		}
	}

	var alarm *models.AlarmRecord
	value := sensorData.SensorValue

	if cfg.DangerThreshold != nil && value >= *cfg.DangerThreshold {
		alarm = &models.AlarmRecord{
			AlarmTime:      sensorData.Time,
			AlarmLevel:     "DANGER",
			AlarmType:      getAlarmType(cfg.SensorType),
			SensorID:       &cfg.SensorID,
			SensorValue:    &value,
			ThresholdValue: cfg.DangerThreshold,
			AlarmMessage: fmt.Sprintf("[危险] %s 值 %.2f %s 超过危险阈值 %.2f %s",
				cfg.SensorName, value, cfg.Unit, *cfg.DangerThreshold, cfg.Unit),
		}
	} else if cfg.WarningThreshold != nil && value >= *cfg.WarningThreshold {
		alarm = &models.AlarmRecord{
			AlarmTime:      sensorData.Time,
			AlarmLevel:     "WARNING",
			AlarmType:      getAlarmType(cfg.SensorType),
			SensorID:       &cfg.SensorID,
			SensorValue:    &value,
			ThresholdValue: cfg.WarningThreshold,
			AlarmMessage: fmt.Sprintf("[预警] %s 值 %.2f %s 超过预警阈值 %.2f %s",
				cfg.SensorName, value, cfg.Unit, *cfg.WarningThreshold, cfg.Unit),
		}
	}

	if alarm != nil {
		ac.cooldowns[cooldownKey] = time.Now()
		mqttTopic := fmt.Sprintf("tashan-weir/alarm/%s", alarm.AlarmLevel)
		alarm.MQTTTopic = &mqttTopic
	}

	return alarm
}

func getAlarmType(sensorType string) string {
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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
