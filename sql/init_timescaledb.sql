-- =====================================================
-- 古代它山堰坝体结构渗流仿真与防渗优化系统
-- TimescaleDB 初始化脚本
-- =====================================================

-- 创建扩展
CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS postgis;

-- =====================================================
-- 1. 坝体基本信息表
-- =====================================================
CREATE TABLE IF NOT EXISTS dam_info (
    id SERIAL PRIMARY KEY,
    dam_name VARCHAR(100) NOT NULL DEFAULT '它山堰',
    build_dynasty VARCHAR(50) DEFAULT '唐代',
    build_year INT DEFAULT 833,
    dam_length NUMERIC(10,2) DEFAULT 113.7,
    dam_height NUMERIC(10,2) DEFAULT 3.85,
    dam_top_width NUMERIC(10,2) DEFAULT 4.8,
    dam_bottom_width NUMERIC(10,2) DEFAULT 12.0,
    upstream_slope NUMERIC(5,3) DEFAULT 0.35,
    downstream_slope NUMERIC(5,3) DEFAULT 0.6,
    design_upstream_water_level NUMERIC(10,2) DEFAULT 8.5,
    design_downstream_water_level NUMERIC(10,2) DEFAULT 3.2,
    material_type VARCHAR(50) DEFAULT '条石-黏土心墙',
    permeability_coefficient NUMERIC(15,10) DEFAULT 1e-7,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO dam_info (dam_name) VALUES ('它山堰') ON CONFLICT DO NOTHING;

-- =====================================================
-- 2. 传感器配置表
-- =====================================================
CREATE TABLE IF NOT EXISTS sensor_config (
    sensor_id VARCHAR(50) PRIMARY KEY,
    sensor_type VARCHAR(30) NOT NULL,
    sensor_name VARCHAR(100),
    location_x NUMERIC(10,3),
    location_y NUMERIC(10,3),
    location_z NUMERIC(10,3),
    installation_date DATE,
    warning_threshold NUMERIC(12,4),
    danger_threshold NUMERIC(12,4),
    unit VARCHAR(20),
    is_active BOOLEAN DEFAULT TRUE,
    dtu_id VARCHAR(50),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO sensor_config (sensor_id, sensor_type, sensor_name, location_x, location_y, location_z, warning_threshold, danger_threshold, unit, dtu_id) VALUES
('PZ-001', 'piezometer', '上游坡脚扬压力计', 2.0, 0.5, 0.0, 60.0, 80.0, 'kPa', 'DTU-TASHAN-001'),
('PZ-002', 'piezometer', '坝心扬压力计1', 28.0, 1.5, 0.0, 55.0, 75.0, 'kPa', 'DTU-TASHAN-001'),
('PZ-003', 'piezometer', '坝心扬压力计2', 56.0, 1.2, 0.0, 50.0, 70.0, 'kPa', 'DTU-TASHAN-001'),
('PZ-004', 'piezometer', '坝心扬压力计3', 84.0, 1.8, 0.0, 55.0, 75.0, 'kPa', 'DTU-TASHAN-001'),
('PZ-005', 'piezometer', '下游坡脚扬压力计', 110.0, 0.3, 0.0, 40.0, 60.0, 'kPa', 'DTU-TASHAN-001'),
('SM-001', 'seepage_meter', '主渗流量监测点', 56.5, 0.0, -0.5, 15.0, 25.0, 'L/s', 'DTU-TASHAN-001'),
('WL-001', 'water_level', '上游水位计', 0.0, 0.0, 6.5, NULL, NULL, 'm', 'DTU-TASHAN-001'),
('WL-002', 'water_level', '下游水位计', 113.7, 0.0, 2.8, NULL, NULL, 'm', 'DTU-TASHAN-001'),
('SD-001', 'scour_depth', '下游河床冲刷深度1', 80.0, 0.0, -1.0, 2.5, 4.0, 'm', 'DTU-TASHAN-001'),
('SD-002', 'scour_depth', '下游河床冲刷深度2', 100.0, 0.0, -0.8, 2.5, 4.0, 'm', 'DTU-TASHAN-001'),
('IL-001', 'infiltration_line', '浸润线测点1', 10.0, 2.0, 0.0, NULL, NULL, 'm', 'DTU-TASHAN-001'),
('IL-002', 'infiltration_line', '浸润线测点2', 30.0, 2.5, 0.0, NULL, NULL, 'm', 'DTU-TASHAN-001'),
('IL-003', 'infiltration_line', '浸润线测点3', 55.0, 2.2, 0.0, NULL, NULL, 'm', 'DTU-TASHAN-001'),
('IL-004', 'infiltration_line', '浸润线测点4', 80.0, 2.8, 0.0, NULL, NULL, 'm', 'DTU-TASHAN-001'),
('IL-005', 'infiltration_line', '浸润线测点5', 100.0, 1.5, 0.0, NULL, NULL, 'm', 'DTU-TASHAN-001')
ON CONFLICT DO NOTHING;

-- =====================================================
-- 3. 传感器数据表 (TimescaleDB hypertable)
-- =====================================================
CREATE TABLE IF NOT EXISTS sensor_data (
    time TIMESTAMPTZ NOT NULL,
    sensor_id VARCHAR(50) NOT NULL,
    sensor_value NUMERIC(14,6) NOT NULL,
    quality INT DEFAULT 0,
    raw_data JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

SELECT create_hypertable('sensor_data', 'time',
    if_not_exists => TRUE,
    chunk_time_interval => INTERVAL '1 day'
);

CREATE INDEX IF NOT EXISTS idx_sensor_data_sensor_time ON sensor_data (sensor_id, time DESC);

-- =====================================================
-- 4. 渗流仿真结果表
-- =====================================================
CREATE TABLE IF NOT EXISTS seepage_simulation (
    id BIGSERIAL PRIMARY KEY,
    simulation_name VARCHAR(200),
    upstream_water_level NUMERIC(10,4) NOT NULL,
    downstream_water_level NUMERIC(10,4) NOT NULL,
    simulation_time TIMESTAMPTZ DEFAULT NOW(),
    total_seepage_flow NUMERIC(15,10),
    max_pore_pressure NUMERIC(12,4),
    grid_count INT,
    calculation_time_ms INT,
    parameters JSONB,
    result_summary JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS seepage_simulation_grid (
    id BIGSERIAL PRIMARY KEY,
    simulation_id BIGINT REFERENCES seepage_simulation(id) ON DELETE CASCADE,
    grid_x NUMERIC(10,4) NOT NULL,
    grid_y NUMERIC(10,4) NOT NULL,
    water_head NUMERIC(12,6),
    pore_pressure NUMERIC(12,4),
    velocity_x NUMERIC(15,10),
    velocity_y NUMERIC(15,10),
    velocity_magnitude NUMERIC(15,10),
    is_saturated BOOLEAN
);

CREATE INDEX IF NOT EXISTS idx_seepage_grid_sim ON seepage_simulation_grid (simulation_id);

-- =====================================================
-- 5. 防渗优化结果表
-- =====================================================
CREATE TABLE IF NOT EXISTS optimization_results (
    id BIGSERIAL PRIMARY KEY,
    optimization_name VARCHAR(200),
    algorithm VARCHAR(50) DEFAULT 'genetic_algorithm',
    upstream_water_level NUMERIC(10,4),
    downstream_water_level NUMERIC(10,4),
    blanket_length NUMERIC(10,4),
    blanket_thickness NUMERIC(10,4),
    blanket_permeability NUMERIC(15,10),
    optimized_seepage_flow NUMERIC(15,10),
    baseline_seepage_flow NUMERIC(15,10),
    flow_reduction_rate NUMERIC(6,3),
    generation_count INT,
    population_size INT,
    best_fitness NUMERIC(15,10),
    optimization_time_ms INT,
    parameters JSONB,
    convergence_curve JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- =====================================================
-- 6. 告警记录表
-- =====================================================
CREATE TABLE IF NOT EXISTS alarm_records (
    id BIGSERIAL PRIMARY KEY,
    alarm_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    alarm_level VARCHAR(20) NOT NULL,
    alarm_type VARCHAR(50) NOT NULL,
    sensor_id VARCHAR(50),
    sensor_value NUMERIC(14,6),
    threshold_value NUMERIC(14,6),
    alarm_message TEXT,
    is_handled BOOLEAN DEFAULT FALSE,
    handled_by VARCHAR(50),
    handled_time TIMESTAMPTZ,
    handle_note TEXT,
    mqtt_published BOOLEAN DEFAULT FALSE,
    mqtt_topic VARCHAR(200),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

SELECT create_hypertable('alarm_records', 'alarm_time',
    if_not_exists => TRUE,
    chunk_time_interval => INTERVAL '1 month'
);

CREATE INDEX IF NOT EXISTS idx_alarm_level ON alarm_records (alarm_level, alarm_time DESC);
CREATE INDEX IF NOT EXISTS idx_alarm_handled ON alarm_records (is_handled, alarm_time DESC);

-- =====================================================
-- 7. 连续查询和聚合视图
-- =====================================================

-- 小时级聚合视图
CREATE MATERIALIZED VIEW IF NOT EXISTS sensor_data_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    sensor_id,
    AVG(sensor_value) AS avg_value,
    MAX(sensor_value) AS max_value,
    MIN(sensor_value) AS min_value,
    LAST(sensor_value, time) AS last_value,
    COUNT(*) AS sample_count
FROM sensor_data
GROUP BY bucket, sensor_id
WITH NO DATA;

-- 日级聚合视图
CREATE MATERIALIZED VIEW IF NOT EXISTS sensor_data_daily
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', time) AS bucket,
    sensor_id,
    AVG(sensor_value) AS avg_value,
    MAX(sensor_value) AS max_value,
    MIN(sensor_value) AS min_value,
    LAST(sensor_value, time) AS last_value,
    COUNT(*) AS sample_count
FROM sensor_data
GROUP BY bucket, sensor_id
WITH NO DATA;

-- =====================================================
-- 8. 辅助函数
-- =====================================================

CREATE OR REPLACE FUNCTION get_recent_sensor_data(p_sensor_id VARCHAR, p_hours INT DEFAULT 24)
RETURNS TABLE (
    r_time TIMESTAMPTZ,
    r_value NUMERIC
) AS $$
BEGIN
    RETURN QUERY
    SELECT time, sensor_value
    FROM sensor_data
    WHERE sensor_id = p_sensor_id
      AND time >= NOW() - (p_hours || ' hours')::INTERVAL
    ORDER BY time;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION calculate_pore_pressure(water_head NUMERIC, elevation NUMERIC)
RETURNS NUMERIC AS $$
BEGIN
    RETURN (water_head - elevation) * 9.81;
END;
$$ LANGUAGE plpgsql IMMUTABLE;
