package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type Config struct {
	APIURL                string  `json:"api_url"`
	DTUID                 string  `json:"dtu_id"`
	IntervalSeconds       int     `json:"interval_seconds"`
	SimSpeed              float64 `json:"sim_speed"`
	UpstreamWL            float64 `json:"upstream_wl"`
	DownstreamWL          float64 `json:"downstream_wl"`
	FoundationPermeability float64 `json:"foundation_permeability"`
	RandomSeed            int64   `json:"random_seed"`
	EnableHTTPAPI         bool    `json:"enable_http_api"`
	HTTPPort              int     `json:"http_port"`
}

type SensorData struct {
	Time        time.Time              `json:"time"`
	SensorID    string                 `json:"sensor_id"`
	SensorValue float64                `json:"sensor_value"`
	Quality     int                    `json:"quality"`
	RawData     map[string]interface{} `json:"raw_data,omitempty"`
}

type DTUPayload struct {
	DTUID     string       `json:"dtu_id"`
	Timestamp time.Time    `json:"timestamp"`
	Sensors   []SensorData `json:"sensors"`
	Signal    float64      `json:"signal_strength"`
	Battery   float64      `json:"battery_level"`
}

type SensorSim struct {
	ID              string
	Type            string
	BaseValue       float64
	OrigBaseValue   float64
	Amplitude       float64
	Phase           float64
	Period          time.Duration
	NoiseLevel      float64
	RandomWalk      float64
	CurrentOffset   float64
	WarningChance   float64
	DangerChance    float64
	WarningMult     float64
	DangerMult      float64
}

type AlarmRequest struct {
	SensorID string `json:"sensor_id"`
	Level    string `json:"level"`
}

var (
	cfg       Config
	sensors   []SensorSim
	rng       *rand.Rand
	cfgMu     sync.RWMutex
	sensorMu  sync.RWMutex
	latestMu  sync.RWMutex
	latest    map[string]SensorData
	baseWL001 float64
	baseWL002 float64
	baseSM001 float64
	basePZ    map[string]float64
)

func init() {
	latest = make(map[string]SensorData)
	basePZ = make(map[string]float64)
}

func loadConfig() {
	cfg = Config{
		APIURL:                getEnv("API_URL", "http://backend:8080/api/v1"),
		DTUID:                 getEnv("DTU_ID", "DTU-TASHAN-001"),
		IntervalSeconds:       getEnvInt("INTERVAL_SECONDS", 600),
		SimSpeed:              getEnvFloat("SIM_SPEED", 1.0),
		UpstreamWL:            getEnvFloat("UPSTREAM_WL", -1),
		DownstreamWL:          getEnvFloat("DOWNSTREAM_WL", -1),
		FoundationPermeability: getEnvFloat("FOUNDATION_PERMEABILITY", -1),
		RandomSeed:            getEnvInt64("RANDOM_SEED", time.Now().UnixNano()),
		EnableHTTPAPI:         getEnvBool("ENABLE_HTTP_API", true),
		HTTPPort:              getEnvInt("HTTP_PORT", 8081),
	}

	if cfg.RandomSeed != 0 {
		rng = rand.New(rand.NewSource(cfg.RandomSeed))
	} else {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
}

func getEnv(key, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultValue
}

func initSensors() {
	sensors = []SensorSim{
		{ID: "PZ-001", Type: "piezometer", BaseValue: 45.0, Amplitude: 8.0, Period: 24 * time.Hour, NoiseLevel: 1.5, RandomWalk: 0.1, WarningChance: 0.02, DangerChance: 0.005, WarningMult: 1.5, DangerMult: 2.0},
		{ID: "PZ-002", Type: "piezometer", BaseValue: 42.0, Amplitude: 7.0, Period: 24 * time.Hour, NoiseLevel: 1.2, RandomWalk: 0.1, WarningChance: 0.02, DangerChance: 0.005, WarningMult: 1.5, DangerMult: 2.0},
		{ID: "PZ-003", Type: "piezometer", BaseValue: 38.0, Amplitude: 6.5, Period: 24 * time.Hour, NoiseLevel: 1.0, RandomWalk: 0.08, WarningChance: 0.02, DangerChance: 0.005, WarningMult: 1.5, DangerMult: 2.0},
		{ID: "PZ-004", Type: "piezometer", BaseValue: 40.0, Amplitude: 7.0, Period: 24 * time.Hour, NoiseLevel: 1.2, RandomWalk: 0.09, WarningChance: 0.02, DangerChance: 0.005, WarningMult: 1.5, DangerMult: 2.0},
		{ID: "PZ-005", Type: "piezometer", BaseValue: 30.0, Amplitude: 5.0, Period: 24 * time.Hour, NoiseLevel: 1.0, RandomWalk: 0.07, WarningChance: 0.015, DangerChance: 0.003, WarningMult: 1.5, DangerMult: 2.0},
		{ID: "SM-001", Type: "seepage_meter", BaseValue: 8.5, Amplitude: 3.0, Period: 12 * time.Hour, NoiseLevel: 0.5, RandomWalk: 0.05, WarningChance: 0.03, DangerChance: 0.01, WarningMult: 2.0, DangerMult: 3.0},
		{ID: "WL-001", Type: "water_level", BaseValue: 6.8, Amplitude: 1.5, Period: 12 * time.Hour, NoiseLevel: 0.05, RandomWalk: 0.01},
		{ID: "WL-002", Type: "water_level", BaseValue: 2.9, Amplitude: 0.5, Period: 12 * time.Hour, NoiseLevel: 0.03, RandomWalk: 0.01},
		{ID: "SD-001", Type: "scour_depth", BaseValue: 1.2, Amplitude: 0.6, Period: 48 * time.Hour, NoiseLevel: 0.1, RandomWalk: 0.02, WarningChance: 0.01, DangerChance: 0.002, WarningMult: 2.5, DangerMult: 4.0},
		{ID: "SD-002", Type: "scour_depth", BaseValue: 1.0, Amplitude: 0.5, Period: 48 * time.Hour, NoiseLevel: 0.08, RandomWalk: 0.02, WarningChance: 0.01, DangerChance: 0.002, WarningMult: 2.5, DangerMult: 4.0},
		{ID: "IL-001", Type: "infiltration_line", BaseValue: 5.8, Amplitude: 0.8, Period: 24 * time.Hour, NoiseLevel: 0.1, RandomWalk: 0.01},
		{ID: "IL-002", Type: "infiltration_line", BaseValue: 5.2, Amplitude: 0.7, Period: 24 * time.Hour, NoiseLevel: 0.08, RandomWalk: 0.01},
		{ID: "IL-003", Type: "infiltration_line", BaseValue: 4.5, Amplitude: 0.6, Period: 24 * time.Hour, NoiseLevel: 0.07, RandomWalk: 0.008},
		{ID: "IL-004", Type: "infiltration_line", BaseValue: 3.8, Amplitude: 0.5, Period: 24 * time.Hour, NoiseLevel: 0.06, RandomWalk: 0.008},
		{ID: "IL-005", Type: "infiltration_line", BaseValue: 2.5, Amplitude: 0.4, Period: 24 * time.Hour, NoiseLevel: 0.05, RandomWalk: 0.006},
	}

	for i := range sensors {
		sensors[i].OrigBaseValue = sensors[i].BaseValue
	}

	baseWL001 = getSensorBase("WL-001")
	baseWL002 = getSensorBase("WL-002")
	baseSM001 = getSensorBase("SM-001")
	for _, id := range []string{"PZ-001", "PZ-002", "PZ-003", "PZ-004", "PZ-005"} {
		basePZ[id] = getSensorBase(id)
	}

	applyConfigOverrides()
}

func getSensorBase(id string) float64 {
	for i := range sensors {
		if sensors[i].ID == id {
			return sensors[i].BaseValue
		}
	}
	return 0
}

func applyConfigOverrides() {
	cfgMu.RLock()
	defer cfgMu.RUnlock()

	if cfg.UpstreamWL >= 0 {
		setSensorBaseValue("WL-001", cfg.UpstreamWL)
		updatePZByUpstreamWL(cfg.UpstreamWL)
	}

	if cfg.DownstreamWL >= 0 {
		setSensorBaseValue("WL-002", cfg.DownstreamWL)
	}

	if cfg.FoundationPermeability >= 0 {
		updateSMByPermeability(cfg.FoundationPermeability)
		updatePZByPermeability(cfg.FoundationPermeability)
	}
}

func setSensorBaseValue(id string, value float64) {
	sensorMu.Lock()
	defer sensorMu.Unlock()
	for i := range sensors {
		if sensors[i].ID == id {
			sensors[i].BaseValue = value
			return
		}
	}
}

func updatePZByUpstreamWL(upstreamWL float64) {
	ratio := upstreamWL / baseWL001
	sensorMu.Lock()
	defer sensorMu.Unlock()
	for i := range sensors {
		if base, ok := basePZ[sensors[i].ID]; ok {
			sensors[i].BaseValue = base * ratio
		}
	}
}

func updateSMByPermeability(permeability float64) {
	defaultPerm := 1e-5
	ratio := permeability / defaultPerm
	sensorMu.Lock()
	defer sensorMu.Unlock()
	for i := range sensors {
		if sensors[i].ID == "SM-001" {
			sensors[i].BaseValue = baseSM001 * ratio
			return
		}
	}
}

func updatePZByPermeability(permeability float64) {
	defaultPerm := 1e-5
	ratio := math.Sqrt(permeability / defaultPerm)
	sensorMu.Lock()
	defer sensorMu.Unlock()
	for i := range sensors {
		if _, ok := basePZ[sensors[i].ID]; ok {
			currentBase := sensors[i].BaseValue
			sensors[i].BaseValue = currentBase * (0.5 + 0.5*ratio)
		}
	}
}

func UpdateConfig(newCfg Config) {
	cfgMu.Lock()
	oldUpstreamWL := cfg.UpstreamWL
	oldDownstreamWL := cfg.DownstreamWL
	oldPermeability := cfg.FoundationPermeability
	cfg = newCfg
	cfgMu.Unlock()

	if newCfg.UpstreamWL >= 0 && newCfg.UpstreamWL != oldUpstreamWL {
		setSensorBaseValue("WL-001", newCfg.UpstreamWL)
		updatePZByUpstreamWL(newCfg.UpstreamWL)
		log.Printf("[CONFIG] Upstream WL updated to %.2fm", newCfg.UpstreamWL)
	}

	if newCfg.DownstreamWL >= 0 && newCfg.DownstreamWL != oldDownstreamWL {
		setSensorBaseValue("WL-002", newCfg.DownstreamWL)
		log.Printf("[CONFIG] Downstream WL updated to %.2fm", newCfg.DownstreamWL)
	}

	if newCfg.FoundationPermeability >= 0 && newCfg.FoundationPermeability != oldPermeability {
		updateSMByPermeability(newCfg.FoundationPermeability)
		updatePZByPermeability(newCfg.FoundationPermeability)
		log.Printf("[CONFIG] Foundation permeability updated to %.2e", newCfg.FoundationPermeability)
	}
}

func simulateSensor(sim *SensorSim, now time.Time, elapsed time.Duration) SensorData {
	t := float64(elapsed) / float64(time.Hour)
	periodHours := float64(sim.Period) / float64(time.Hour)

	sim.CurrentOffset += rng.NormFloat64() * sim.RandomWalk * 0.1
	sim.CurrentOffset = clamp(sim.CurrentOffset, -sim.Amplitude*0.5, sim.Amplitude*0.5)

	cyclic := sim.Amplitude * math.Sin(2*math.Pi*t/periodHours+sim.Phase)
	noise := rng.NormFloat64() * sim.NoiseLevel

	value := sim.BaseValue + cyclic + noise + sim.CurrentOffset

	if sim.WarningChance > 0 && rng.Float64() < sim.WarningChance {
		value = sim.BaseValue * sim.WarningMult
		log.Printf("[WARN] %s triggered warning event, value=%.2f", sim.ID, value)
	}

	if sim.DangerChance > 0 && rng.Float64() < sim.DangerChance {
		value = sim.BaseValue * sim.DangerMult
		log.Printf("[DANGER] %s triggered danger event, value=%.2f", sim.ID, value)
	}

	if value < 0 {
		value = 0
	}

	quality := 0
	if math.Abs(noise) > sim.NoiseLevel*3 {
		quality = 1
	}

	return SensorData{
		Time:        now,
		SensorID:    sim.ID,
		SensorValue: roundTo(value, 4),
		Quality:     quality,
		RawData: map[string]interface{}{
			"base_value":     roundTo(sim.BaseValue, 4),
			"cyclic_part":    roundTo(cyclic, 4),
			"noise_part":     roundTo(noise, 4),
			"drift_offset":   roundTo(sim.CurrentOffset, 4),
			"simulated":      true,
			"dtu_channel":    getDTUChannel(sim.ID),
		},
	}
}

func getDTUChannel(sensorID string) int {
	channelMap := map[string]int{
		"PZ-001": 1, "PZ-002": 2, "PZ-003": 3, "PZ-004": 4, "PZ-005": 5,
		"SM-001": 6, "WL-001": 7, "WL-002": 8, "SD-001": 9, "SD-002": 10,
		"IL-001": 11, "IL-002": 12, "IL-003": 13, "IL-004": 14, "IL-005": 15,
	}
	if ch, ok := channelMap[sensorID]; ok {
		return ch
	}
	return 0
}

func sendData(payload DTUPayload) error {
	cfgMu.RLock()
	apiURL := cfg.APIURL
	dtuID := cfg.DTUID
	cfgMu.RUnlock()

	url := apiURL + "/dtu/data"
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DTU-ID", dtuID)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if alarms, ok := result["alarms"]; ok {
		if alarmList, ok := alarms.([]interface{}); ok && len(alarmList) > 0 {
			log.Printf("[ALERT] %d alarms generated", len(alarmList))
		}
	}

	return nil
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

func handlePostConfig(w http.ResponseWriter, r *http.Request) {
	var newCfg Config
	if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	cfgMu.RLock()
	merged := cfg
	if newCfg.APIURL != "" {
		merged.APIURL = newCfg.APIURL
	}
	if newCfg.DTUID != "" {
		merged.DTUID = newCfg.DTUID
	}
	if newCfg.IntervalSeconds > 0 {
		merged.IntervalSeconds = newCfg.IntervalSeconds
	}
	if newCfg.SimSpeed > 0 {
		merged.SimSpeed = newCfg.SimSpeed
	}
	if newCfg.UpstreamWL >= 0 {
		merged.UpstreamWL = newCfg.UpstreamWL
	}
	if newCfg.DownstreamWL >= 0 {
		merged.DownstreamWL = newCfg.DownstreamWL
	}
	if newCfg.FoundationPermeability >= 0 {
		merged.FoundationPermeability = newCfg.FoundationPermeability
	}
	if newCfg.HTTPPort > 0 {
		merged.HTTPPort = newCfg.HTTPPort
	}
	cfgMu.RUnlock()

	UpdateConfig(merged)

	w.Header().Set("Content-Type", "application/json")
	cfgMu.RLock()
	json.NewEncoder(w).Encode(cfg)
	cfgMu.RUnlock()
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	latestMu.RLock()
	defer latestMu.RUnlock()

	result := make(map[string]interface{})
	result["time"] = time.Now().Format(time.RFC3339)
	sensorsMap := make(map[string]SensorData)
	for k, v := range latest {
		sensorsMap[k] = v
	}
	result["sensors"] = sensorsMap

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleTriggerAlarm(w http.ResponseWriter, r *http.Request) {
	var req AlarmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if req.SensorID == "" {
		http.Error(w, `{"error":"sensor_id is required"}`, http.StatusBadRequest)
		return
	}
	if req.Level != "warning" && req.Level != "danger" {
		http.Error(w, `{"error":"level must be 'warning' or 'danger'"}`, http.StatusBadRequest)
		return
	}

	sensorMu.Lock()
	found := false
	var triggeredValue float64
	for i := range sensors {
		if sensors[i].ID == req.SensorID {
			found = true
			if req.Level == "warning" {
				triggeredValue = sensors[i].BaseValue * sensors[i].WarningMult
			} else {
				triggeredValue = sensors[i].BaseValue * sensors[i].DangerMult
			}

			data := SensorData{
				Time:        time.Now(),
				SensorID:    sensors[i].ID,
				SensorValue: roundTo(triggeredValue, 4),
				Quality:     0,
				RawData: map[string]interface{}{
					"base_value":  roundTo(sensors[i].BaseValue, 4),
					"simulated":   true,
					"alarm_level": req.Level,
					"manual":      true,
					"dtu_channel": getDTUChannel(sensors[i].ID),
				},
			}
			latestMu.Lock()
			latest[sensors[i].ID] = data
			latestMu.Unlock()

			cfgMu.RLock()
			dtuID := cfg.DTUID
			cfgMu.RUnlock()

			payload := DTUPayload{
				DTUID:     dtuID,
				Timestamp: time.Now(),
				Sensors:   []SensorData{data},
				Signal:    -65 + rng.Float64()*15,
				Battery:   85 + rng.Float64()*15,
			}
			go sendData(payload)

			log.Printf("[MANUAL ALARM] %s %s triggered, value=%.2f", req.SensorID, req.Level, triggeredValue)
			break
		}
	}
	sensorMu.Unlock()

	if !found {
		http.Error(w, `{"error":"sensor not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "triggered",
		"sensor_id": req.SensorID,
		"level":     req.Level,
		"value":     triggeredValue,
	})
}

func startHTTPServer() {
	cfgMu.RLock()
	enableAPI := cfg.EnableHTTPAPI
	port := cfg.HTTPPort
	cfgMu.RUnlock()

	if !enableAPI {
		log.Println("HTTP API disabled by configuration")
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handleGetConfig(w, r)
		} else if r.Method == http.MethodPost {
			handlePostConfig(w, r)
		} else {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/status", handleStatus)
	mux.HandleFunc("/trigger-alarm", handleTriggerAlarm)

	addr := fmt.Sprintf(":%d", port)
	log.Printf("HTTP API server starting on %s", addr)
	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Printf("[ERROR] HTTP server failed: %v", err)
		}
	}()
}

func main() {
	loadConfig()
	initSensors()

	log.Println("================================================")
	log.Println("  它山堰 DTU 传感器模拟器")
	log.Println("  Tashan Weir Sensor Simulator")
	log.Println("================================================")
	cfgMu.RLock()
	log.Printf("DTU ID: %s", cfg.DTUID)
	log.Printf("API Endpoint: %s/dtu/data", cfg.APIURL)
	log.Printf("Interval: %d seconds", cfg.IntervalSeconds)
	log.Printf("Sim Speed: %.1fx", cfg.SimSpeed)
	log.Printf("Sensors: %d", len(sensors))
	if cfg.UpstreamWL >= 0 {
		log.Printf("Upstream WL: %.2fm (override)", cfg.UpstreamWL)
	}
	if cfg.DownstreamWL >= 0 {
		log.Printf("Downstream WL: %.2fm (override)", cfg.DownstreamWL)
	}
	if cfg.FoundationPermeability >= 0 {
		log.Printf("Foundation Permeability: %.2e (override)", cfg.FoundationPermeability)
	}
	log.Printf("HTTP API: enabled=%v port=%d", cfg.EnableHTTPAPI, cfg.HTTPPort)
	cfgMu.RUnlock()
	log.Println()

	startHTTPServer()

	cfgMu.RLock()
	simSpeed := cfg.SimSpeed
	intervalSec := cfg.IntervalSeconds
	cfgMu.RUnlock()

	simInterval := time.Duration(float64(intervalSec)*1000/float64(simSpeed)) * time.Millisecond
	startTime := time.Now()
	cycleCount := 0

	log.Println("Simulator started. Press Ctrl+C to stop.")
	log.Println()

	for {
		cycleCount++
		now := time.Now()

		cfgMu.RLock()
		currentSimSpeed := cfg.SimSpeed
		dtuID := cfg.DTUID
		cfgMu.RUnlock()

		elapsed := now.Sub(startTime) * time.Duration(currentSimSpeed)
		simTime := startTime.Add(elapsed)

		payload := DTUPayload{
			DTUID:     dtuID,
			Timestamp: simTime,
			Sensors:   make([]SensorData, 0, len(sensors)),
			Signal:    -65 + rng.Float64()*15,
			Battery:   85 + rng.Float64()*15,
		}

		sensorMu.Lock()
		for i := range sensors {
			sensorData := simulateSensor(&sensors[i], simTime, elapsed)
			payload.Sensors = append(payload.Sensors, sensorData)

			latestMu.Lock()
			latest[sensorData.SensorID] = sensorData
			latestMu.Unlock()
		}
		sensorMu.Unlock()

		err := sendData(payload)
		if err != nil {
			log.Printf("[ERROR] Cycle %d failed: %v", cycleCount, err)
		} else {
			log.Printf("[OK] Cycle %d | %s | %d sensors | Sig:%.1fdBm Bat:%.0f%%",
				cycleCount,
				simTime.Format("2006-01-02 15:04:05"),
				len(payload.Sensors),
				payload.Signal,
				payload.Battery,
			)

			summary := make(map[string]float64)
			for _, s := range payload.Sensors {
				summary[s.SensorID] = s.SensorValue
			}
			log.Printf("    WL-001=%.2fm WL-002=%.2fm SM-001=%.2fL/s PZ-003=%.1fkPa SD-001=%.2fm",
				summary["WL-001"], summary["WL-002"],
				summary["SM-001"], summary["PZ-003"], summary["SD-001"])
		}

		log.Println()

		cfgMu.RLock()
		latestIntervalSec := cfg.IntervalSeconds
		latestSimSpeed := cfg.SimSpeed
		cfgMu.RUnlock()
		simInterval = time.Duration(float64(latestIntervalSec)*1000/float64(latestSimSpeed)) * time.Millisecond

		time.Sleep(simInterval)
	}
}

func clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func roundTo(val float64, decimals int) float64 {
	ratio := math.Pow(10, float64(decimals))
	return math.Round(val*ratio) / ratio
}
