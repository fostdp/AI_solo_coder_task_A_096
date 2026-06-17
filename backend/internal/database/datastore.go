package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"tashan-weir-seepage/internal/models"
)

type DataStore struct {
	db *Database
}

func NewDataStore(db *Database) *DataStore {
	return &DataStore{db: db}
}

func (ds *DataStore) InsertSensorData(ctx context.Context, data *models.SensorData) error {
	var rawDataJSON []byte
	if data.RawData != nil {
		rawDataJSON, _ = json.Marshal(data.RawData)
	}

	query := `INSERT INTO sensor_data (time, sensor_id, sensor_value, quality, raw_data) 
		VALUES ($1, $2, $3, $4, $5)`
	_, err := ds.db.Pool().Exec(ctx, query,
		data.Time, data.SensorID, data.SensorValue, data.Quality, rawDataJSON)
	return err
}

func (ds *DataStore) InsertSensorDataBatch(ctx context.Context, dataList []models.SensorData) error {
	tx, err := ds.db.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	stmt := `INSERT INTO sensor_data (time, sensor_id, sensor_value, quality, raw_data) 
		VALUES ($1, $2, $3, $4, $5)`

	for _, data := range dataList {
		var rawDataJSON []byte
		if data.RawData != nil {
			rawDataJSON, _ = json.Marshal(data.RawData)
		}
		_, err := tx.Exec(ctx, stmt,
			data.Time, data.SensorID, data.SensorValue, data.Quality, rawDataJSON)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (ds *DataStore) GetRecentSensorData(ctx context.Context, sensorID string, hours int) ([]models.SensorData, error) {
	query := `SELECT time, sensor_id, sensor_value, quality, raw_data, created_at 
		FROM sensor_data 
		WHERE sensor_id = $1 AND time >= NOW() - $2::INTERVAL 
		ORDER BY time`
	interval := fmt.Sprintf("%d hours", hours)

	rows, err := ds.db.Pool().Query(ctx, query, sensorID, interval)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.SensorData
	for rows.Next() {
		var d models.SensorData
		var rawDataJSON []byte
		err := rows.Scan(&d.Time, &d.SensorID, &d.SensorValue, &d.Quality, &rawDataJSON, &d.CreatedAt)
		if err != nil {
			return nil, err
		}
		if len(rawDataJSON) > 0 {
			json.Unmarshal(rawDataJSON, &d.RawData)
		}
		results = append(results, d)
	}
	return results, nil
}

func (ds *DataStore) GetLatestSensorValues(ctx context.Context) ([]models.SensorData, error) {
	query := `SELECT DISTINCT ON (sensor_id) time, sensor_id, sensor_value, quality, raw_data, created_at 
		FROM sensor_data 
		ORDER BY sensor_id, time DESC`

	rows, err := ds.db.Pool().Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.SensorData
	for rows.Next() {
		var d models.SensorData
		var rawDataJSON []byte
		err := rows.Scan(&d.Time, &d.SensorID, &d.SensorValue, &d.Quality, &rawDataJSON, &d.CreatedAt)
		if err != nil {
			return nil, err
		}
		if len(rawDataJSON) > 0 {
			json.Unmarshal(rawDataJSON, &d.RawData)
		}
		results = append(results, d)
	}
	return results, nil
}

func (ds *DataStore) GetAllSensorConfigs(ctx context.Context) ([]models.SensorConfig, error) {
	query := `SELECT sensor_id, sensor_type, sensor_name, location_x, location_y, location_z,
		installation_date, warning_threshold, danger_threshold, unit, is_active, dtu_id, created_at
		FROM sensor_config WHERE is_active = TRUE ORDER BY sensor_id`

	rows, err := ds.db.Pool().Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.SensorConfig
	for rows.Next() {
		var s models.SensorConfig
		var instDate *time.Time
		err := rows.Scan(&s.SensorID, &s.SensorType, &s.SensorName,
			&s.LocationX, &s.LocationY, &s.LocationZ,
			&instDate, &s.WarningThreshold, &s.DangerThreshold,
			&s.Unit, &s.IsActive, &s.DTUID, &s.CreatedAt)
		if err != nil {
			return nil, err
		}
		if instDate != nil {
			s.InstallationDate = instDate.Format("2006-01-02")
		}
		results = append(results, s)
	}
	return results, nil
}

func (ds *DataStore) InsertAlarm(ctx context.Context, alarm *models.AlarmRecord) (int64, error) {
	query := `INSERT INTO alarm_records 
		(alarm_time, alarm_level, alarm_type, sensor_id, sensor_value, threshold_value, 
		 alarm_message, mqtt_topic)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`

	var id int64
	err := ds.db.Pool().QueryRow(ctx, query,
		alarm.AlarmTime, alarm.AlarmLevel, alarm.AlarmType,
		alarm.SensorID, alarm.SensorValue, alarm.ThresholdValue,
		alarm.AlarmMessage, alarm.MQTTTopic).Scan(&id)
	return id, err
}

func (ds *DataStore) UpdateAlarmMQTTPublished(ctx context.Context, alarmID int64) error {
	query := `UPDATE alarm_records SET mqtt_published = TRUE WHERE id = $1`
	_, err := ds.db.Pool().Exec(ctx, query, alarmID)
	return err
}

func (ds *DataStore) GetRecentAlarms(ctx context.Context, limit int, onlyUnhandled bool) ([]models.AlarmRecord, error) {
	var query string
	if onlyUnhandled {
		query = `SELECT id, alarm_time, alarm_level, alarm_type, sensor_id, sensor_value, 
			threshold_value, alarm_message, is_handled, handled_by, handled_time, 
			handle_note, mqtt_published, mqtt_topic, created_at
			FROM alarm_records WHERE is_handled = FALSE ORDER BY alarm_time DESC LIMIT $1`
	} else {
		query = `SELECT id, alarm_time, alarm_level, alarm_type, sensor_id, sensor_value, 
			threshold_value, alarm_message, is_handled, handled_by, handled_time, 
			handle_note, mqtt_published, mqtt_topic, created_at
			FROM alarm_records ORDER BY alarm_time DESC LIMIT $1`
	}

	rows, err := ds.db.Pool().Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.AlarmRecord
	for rows.Next() {
		var a models.AlarmRecord
		err := rows.Scan(&a.ID, &a.AlarmTime, &a.AlarmLevel, &a.AlarmType,
			&a.SensorID, &a.SensorValue, &a.ThresholdValue, &a.AlarmMessage,
			&a.IsHandled, &a.HandledBy, &a.HandledTime, &a.HandleNote,
			&a.MQTTPublished, &a.MQTTTopic, &a.CreatedAt)
		if err != nil {
			return nil, err
		}
		results = append(results, a)
	}
	return results, nil
}

func (ds *DataStore) InsertSimulation(ctx context.Context, sim *models.SeepageSimulation) (int64, error) {
	paramsJSON, _ := json.Marshal(sim.Parameters)
	summaryJSON, _ := json.Marshal(sim.ResultSummary)

	query := `INSERT INTO seepage_simulation 
		(simulation_name, upstream_water_level, downstream_water_level, 
		 total_seepage_flow, max_pore_pressure, grid_count, calculation_time_ms,
		 parameters, result_summary)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`

	var id int64
	err := ds.db.Pool().QueryRow(ctx, query,
		sim.SimulationName, sim.UpstreamWaterLevel, sim.DownstreamWaterLevel,
		sim.TotalSeepageFlow, sim.MaxPorePressure, sim.GridCount,
		sim.CalculationTimeMs, paramsJSON, summaryJSON).Scan(&id)
	return id, err
}

func (ds *DataStore) InsertSimulationGrids(ctx context.Context, simulationID int64, grids []models.SimulationGrid) error {
	tx, err := ds.db.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	stmt := `INSERT INTO seepage_simulation_grid 
		(simulation_id, grid_x, grid_y, water_head, pore_pressure, 
		 velocity_x, velocity_y, velocity_magnitude, is_saturated)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	for _, g := range grids {
		_, err := tx.Exec(ctx, stmt,
			simulationID, g.GridX, g.GridY, g.WaterHead, g.PorePressure,
			g.VelocityX, g.VelocityY, g.VelocityMagnitude, g.IsSaturated)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (ds *DataStore) GetSimulation(ctx context.Context, simulationID int64) (*models.SeepageSimulation, error) {
	query := `SELECT id, simulation_name, upstream_water_level, downstream_water_level,
		simulation_time, total_seepage_flow, max_pore_pressure, grid_count,
		calculation_time_ms, parameters, result_summary, created_at
		FROM seepage_simulation WHERE id = $1`

	var sim models.SeepageSimulation
	var paramsJSON, summaryJSON []byte

	err := ds.db.Pool().QueryRow(ctx, query, simulationID).Scan(
		&sim.ID, &sim.SimulationName, &sim.UpstreamWaterLevel, &sim.DownstreamWaterLevel,
		&sim.SimulationTime, &sim.TotalSeepageFlow, &sim.MaxPorePressure, &sim.GridCount,
		&sim.CalculationTimeMs, &paramsJSON, &summaryJSON, &sim.CreatedAt)
	if err != nil {
		return nil, err
	}

	if len(paramsJSON) > 0 {
		json.Unmarshal(paramsJSON, &sim.Parameters)
	}
	if len(summaryJSON) > 0 {
		json.Unmarshal(summaryJSON, &sim.ResultSummary)
	}

	return &sim, nil
}

func (ds *DataStore) GetSimulationGrids(ctx context.Context, simulationID int64) ([]models.SimulationGrid, error) {
	query := `SELECT id, simulation_id, grid_x, grid_y, water_head, pore_pressure,
		velocity_x, velocity_y, velocity_magnitude, is_saturated
		FROM seepage_simulation_grid WHERE simulation_id = $1 ORDER BY grid_y, grid_x`

	rows, err := ds.db.Pool().Query(ctx, query, simulationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.SimulationGrid
	for rows.Next() {
		var g models.SimulationGrid
		err := rows.Scan(&g.ID, &g.SimulationID, &g.GridX, &g.GridY,
			&g.WaterHead, &g.PorePressure, &g.VelocityX, &g.VelocityY,
			&g.VelocityMagnitude, &g.IsSaturated)
		if err != nil {
			return nil, err
		}
		results = append(results, g)
	}
	return results, nil
}

func (ds *DataStore) GetSimulations(ctx context.Context, limit int) ([]models.SeepageSimulation, error) {
	query := `SELECT id, simulation_name, upstream_water_level, downstream_water_level,
		simulation_time, total_seepage_flow, max_pore_pressure, grid_count,
		calculation_time_ms, parameters, result_summary, created_at
		FROM seepage_simulation ORDER BY simulation_time DESC LIMIT $1`

	rows, err := ds.db.Pool().Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.SeepageSimulation
	for rows.Next() {
		var sim models.SeepageSimulation
		var paramsJSON, summaryJSON []byte
		err := rows.Scan(&sim.ID, &sim.SimulationName, &sim.UpstreamWaterLevel, &sim.DownstreamWaterLevel,
			&sim.SimulationTime, &sim.TotalSeepageFlow, &sim.MaxPorePressure, &sim.GridCount,
			&sim.CalculationTimeMs, &paramsJSON, &summaryJSON, &sim.CreatedAt)
		if err != nil {
			return nil, err
		}
		if len(paramsJSON) > 0 {
			json.Unmarshal(paramsJSON, &sim.Parameters)
		}
		if len(summaryJSON) > 0 {
			json.Unmarshal(summaryJSON, &sim.ResultSummary)
		}
		results = append(results, sim)
	}
	return results, nil
}

func (ds *DataStore) InsertOptimizationResult(ctx context.Context, opt *models.OptimizationResult) (int64, error) {
	paramsJSON, _ := json.Marshal(opt.Parameters)
	convergenceJSON, _ := json.Marshal(opt.ConvergenceCurve)

	query := `INSERT INTO optimization_results
		(optimization_name, algorithm, upstream_water_level, downstream_water_level,
		 blanket_length, blanket_thickness, blanket_permeability,
		 optimized_seepage_flow, baseline_seepage_flow, flow_reduction_rate,
		 generation_count, population_size, best_fitness, optimization_time_ms,
		 parameters, convergence_curve)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16) RETURNING id`

	var id int64
	err := ds.db.Pool().QueryRow(ctx, query,
		opt.OptimizationName, opt.Algorithm, opt.UpstreamWaterLevel, opt.DownstreamWaterLevel,
		opt.BlanketLength, opt.BlanketThickness, opt.BlanketPermeability,
		opt.OptimizedSeepageFlow, opt.BaselineSeepageFlow, opt.FlowReductionRate,
		opt.GenerationCount, opt.PopulationSize, opt.BestFitness, opt.OptimizationTimeMs,
		paramsJSON, convergenceJSON).Scan(&id)
	return id, err
}

func (ds *DataStore) GetOptimizationResults(ctx context.Context, limit int) ([]models.OptimizationResult, error) {
	query := `SELECT id, optimization_name, algorithm, upstream_water_level, downstream_water_level,
		blanket_length, blanket_thickness, blanket_permeability,
		optimized_seepage_flow, baseline_seepage_flow, flow_reduction_rate,
		generation_count, population_size, best_fitness, optimization_time_ms,
		parameters, convergence_curve, created_at
		FROM optimization_results ORDER BY created_at DESC LIMIT $1`

	rows, err := ds.db.Pool().Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.OptimizationResult
	for rows.Next() {
		var opt models.OptimizationResult
		var paramsJSON, convergenceJSON []byte
		err := rows.Scan(&opt.ID, &opt.OptimizationName, &opt.Algorithm,
			&opt.UpstreamWaterLevel, &opt.DownstreamWaterLevel,
			&opt.BlanketLength, &opt.BlanketThickness, &opt.BlanketPermeability,
			&opt.OptimizedSeepageFlow, &opt.BaselineSeepageFlow, &opt.FlowReductionRate,
			&opt.GenerationCount, &opt.PopulationSize, &opt.BestFitness,
			&opt.OptimizationTimeMs, &paramsJSON, &convergenceJSON, &opt.CreatedAt)
		if err != nil {
			return nil, err
		}
		if len(paramsJSON) > 0 {
			json.Unmarshal(paramsJSON, &opt.Parameters)
		}
		if len(convergenceJSON) > 0 {
			json.Unmarshal(convergenceJSON, &opt.ConvergenceCurve)
		}
		results = append(results, opt)
	}
	return results, nil
}

func (ds *DataStore) GetDamInfo(ctx context.Context) (*models.DamInfo, error) {
	query := `SELECT id, dam_name, build_dynasty, build_year, dam_length, dam_height,
		dam_top_width, dam_bottom_width, upstream_slope, downstream_slope,
		design_upstream_water_level, design_downstream_water_level,
		material_type, permeability_coefficient, created_at, updated_at
		FROM dam_info ORDER BY id LIMIT 1`

	var d models.DamInfo
	err := ds.db.Pool().QueryRow(ctx, query).Scan(
		&d.ID, &d.DamName, &d.BuildDynasty, &d.BuildYear,
		&d.DamLength, &d.DamHeight, &d.DamTopWidth, &d.DamBottomWidth,
		&d.UpstreamSlope, &d.DownstreamSlope,
		&d.DesignUpstreamWaterLevel, &d.DesignDownstreamWaterLevel,
		&d.MaterialType, &d.PermeabilityCoefficient, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}
