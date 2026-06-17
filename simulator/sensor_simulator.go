package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"time"
)

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

var sensors = []SensorSim{
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

const (
	apiBaseURL    = "http://localhost:8080/api/v1"
	dtuID         = "DTU-TASHAN-001"
	intervalMin   = 10
	simSpeedMult  = 1
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

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
			"base_value":     sim.BaseValue,
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
	url := apiBaseURL + "/dtu/data"
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

func main() {
	log.Println("================================================")
	log.Println("  它山堰 DTU 传感器模拟器")
	log.Println("  Tashan Weir Sensor Simulator")
	log.Println("================================================")
	log.Printf("DTU ID: %s", dtuID)
	log.Printf("API Endpoint: %s/dtu/data", apiBaseURL)
	log.Printf("Interval: %d minutes", intervalMin)
	log.Printf("Sensors: %d", len(sensors))
	log.Println()

	simInterval := time.Duration(intervalMin) * time.Minute / time.Duration(simSpeedMult)
	startTime := time.Now()
	cycleCount := 0

	log.Println("Simulator started. Press Ctrl+C to stop.")
	log.Println()

	for {
		cycleCount++
		now := time.Now()
		elapsed := now.Sub(startTime) * time.Duration(simSpeedMult)
		simTime := startTime.Add(elapsed)

		payload := DTUPayload{
			DTUID:     dtuID,
			Timestamp: simTime,
			Sensors:   make([]SensorData, 0, len(sensors)),
			Signal:    -65 + rng.Float64()*15,
			Battery:   85 + rng.Float64()*15,
		}

		for i := range sensors {
			sensorData := simulateSensor(&sensors[i], simTime, elapsed)
			payload.Sensors = append(payload.Sensors, sensorData)
		}

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
