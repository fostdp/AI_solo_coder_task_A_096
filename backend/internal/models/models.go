package models

import (
	"time"
)

type DamInfo struct {
	ID                      int       `json:"id"`
	DamName                 string    `json:"dam_name"`
	BuildDynasty            string    `json:"build_dynasty"`
	BuildYear               int       `json:"build_year"`
	DamLength               float64   `json:"dam_length"`
	DamHeight               float64   `json:"dam_height"`
	DamTopWidth             float64   `json:"dam_top_width"`
	DamBottomWidth          float64   `json:"dam_bottom_width"`
	UpstreamSlope           float64   `json:"upstream_slope"`
	DownstreamSlope         float64   `json:"downstream_slope"`
	DesignUpstreamWaterLevel float64  `json:"design_upstream_water_level"`
	DesignDownstreamWaterLevel float64 `json:"design_downstream_water_level"`
	MaterialType            string    `json:"material_type"`
	PermeabilityCoefficient float64   `json:"permeability_coefficient"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

type SensorConfig struct {
	SensorID          string    `json:"sensor_id"`
	SensorType        string    `json:"sensor_type"`
	SensorName        string    `json:"sensor_name"`
	LocationX         float64   `json:"location_x"`
	LocationY         float64   `json:"location_y"`
	LocationZ         float64   `json:"location_z"`
	InstallationDate  string    `json:"installation_date"`
	WarningThreshold  *float64  `json:"warning_threshold"`
	DangerThreshold   *float64  `json:"danger_threshold"`
	Unit              string    `json:"unit"`
	IsActive          bool      `json:"is_active"`
	DTUID             string    `json:"dtu_id"`
	CreatedAt         time.Time `json:"created_at"`
}

type SensorData struct {
	Time        time.Time            `json:"time"`
	SensorID    string               `json:"sensor_id"`
	SensorValue float64              `json:"sensor_value"`
	Quality     int                  `json:"quality"`
	RawData     map[string]interface{} `json:"raw_data,omitempty"`
	CreatedAt   time.Time            `json:"created_at"`
}

type DTUPayload struct {
	DTUID     string       `json:"dtu_id"`
	Timestamp time.Time    `json:"timestamp"`
	Sensors   []SensorData `json:"sensors"`
	Signal    float64      `json:"signal_strength"`
	Battery   float64      `json:"battery_level"`
}

type AlarmRecord struct {
	ID             int64     `json:"id"`
	AlarmTime      time.Time `json:"alarm_time"`
	AlarmLevel     string    `json:"alarm_level"`
	AlarmType      string    `json:"alarm_type"`
	SensorID       *string   `json:"sensor_id"`
	SensorValue    *float64  `json:"sensor_value"`
	ThresholdValue *float64  `json:"threshold_value"`
	AlarmMessage   string    `json:"alarm_message"`
	IsHandled      bool      `json:"is_handled"`
	HandledBy      *string   `json:"handled_by"`
	HandledTime    *time.Time `json:"handled_time"`
	HandleNote     *string   `json:"handle_note"`
	MQTTPublished  bool      `json:"mqtt_published"`
	MQTTTopic      *string   `json:"mqtt_topic"`
	CreatedAt      time.Time `json:"created_at"`
}

type SeepageSimulation struct {
	ID                  int64                  `json:"id"`
	SimulationName      string                 `json:"simulation_name"`
	UpstreamWaterLevel  float64                `json:"upstream_water_level"`
	DownstreamWaterLevel float64               `json:"downstream_water_level"`
	SimulationTime      time.Time              `json:"simulation_time"`
	TotalSeepageFlow    float64                `json:"total_seepage_flow"`
	MaxPorePressure     float64                `json:"max_pore_pressure"`
	GridCount           int                    `json:"grid_count"`
	CalculationTimeMs   int64                  `json:"calculation_time_ms"`
	Parameters          map[string]interface{} `json:"parameters"`
	ResultSummary       map[string]interface{} `json:"result_summary"`
	CreatedAt           time.Time              `json:"created_at"`
}

type SimulationGrid struct {
	ID                int64   `json:"id"`
	SimulationID      int64   `json:"simulation_id"`
	GridX             float64 `json:"grid_x"`
	GridY             float64 `json:"grid_y"`
	WaterHead         float64 `json:"water_head"`
	PorePressure      float64 `json:"pore_pressure"`
	VelocityX         float64 `json:"velocity_x"`
	VelocityY         float64 `json:"velocity_y"`
	VelocityMagnitude float64 `json:"velocity_magnitude"`
	IsSaturated       bool    `json:"is_saturated"`
}

type OptimizationResult struct {
	ID                    int64                  `json:"id"`
	OptimizationName      string                 `json:"optimization_name"`
	Algorithm             string                 `json:"algorithm"`
	UpstreamWaterLevel    float64                `json:"upstream_water_level"`
	DownstreamWaterLevel  float64                `json:"downstream_water_level"`
	BlanketLength         float64                `json:"blanket_length"`
	BlanketThickness      float64                `json:"blanket_thickness"`
	BlanketPermeability   float64                `json:"blanket_permeability"`
	OptimizedSeepageFlow  float64                `json:"optimized_seepage_flow"`
	BaselineSeepageFlow   float64                `json:"baseline_seepage_flow"`
	FlowReductionRate     float64                `json:"flow_reduction_rate"`
	GenerationCount       int                    `json:"generation_count"`
	PopulationSize        int                    `json:"population_size"`
	BestFitness           float64                `json:"best_fitness"`
	OptimizationTimeMs    int64                  `json:"optimization_time_ms"`
	Parameters            map[string]interface{} `json:"parameters"`
	ConvergenceCurve      []float64              `json:"convergence_curve"`
	CreatedAt             time.Time              `json:"created_at"`
}

type SimulationRequest struct {
	UpstreamWaterLevel   float64                `json:"upstream_water_level"`
	DownstreamWaterLevel float64                `json:"downstream_water_level"`
	GridResolutionX      int                    `json:"grid_resolution_x"`
	GridResolutionY      int                    `json:"grid_resolution_y"`
	PermeabilityK        float64                `json:"permeability_k"`
	BlanketLength        *float64               `json:"blanket_length,omitempty"`
	BlanketThickness     *float64               `json:"blanket_thickness,omitempty"`
	BlanketPermeability  *float64               `json:"blanket_permeability,omitempty"`
	SimulationName       string                 `json:"simulation_name"`
	Parameters           map[string]interface{} `json:"parameters"`
}

type OptimizationRequest struct {
	UpstreamWaterLevel   float64 `json:"upstream_water_level"`
	DownstreamWaterLevel float64 `json:"downstream_water_level"`
	MinBlanketLength     float64 `json:"min_blanket_length"`
	MaxBlanketLength     float64 `json:"max_blanket_length"`
	MinBlanketThickness  float64 `json:"min_blanket_thickness"`
	MaxBlanketThickness  float64 `json:"max_blanket_thickness"`
	PopulationSize       int     `json:"population_size"`
	MaxGenerations       int     `json:"max_generations"`
	MutationRate         float64 `json:"mutation_rate"`
	CrossoverRate        float64 `json:"crossover_rate"`
	OptimizationName     string  `json:"optimization_name"`
}

type ParetoSolution struct {
	BlanketLength    float64 `json:"blanket_length"`
	BlanketThickness float64 `json:"blanket_thickness"`
	SeepageFlow      float64 `json:"seepage_flow"`
	MaterialCost     float64 `json:"material_cost"`
	FlowReduction    float64 `json:"flow_reduction"`
	Rank             int     `json:"rank"`
}
