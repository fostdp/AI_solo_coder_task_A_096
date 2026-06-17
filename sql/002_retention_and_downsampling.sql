-- =====================================================
-- TimescaleDB 数据保留策略和降采样配置
-- Data Retention Policies and Downsampling
-- =====================================================

-- =====================================================
-- 1. 数据保留策略 (Data Retention Policies)
-- =====================================================

-- 原始传感器数据：保留7天
SELECT add_retention_policy('sensor_data', INTERVAL '7 days', if_not_exists => TRUE);

-- 小时级聚合数据：保留90天
SELECT add_retention_policy('sensor_data_hourly', INTERVAL '90 days', if_not_exists => TRUE);

-- 日级聚合数据：保留2年
SELECT add_retention_policy('sensor_data_daily', INTERVAL '2 years', if_not_exists => TRUE);

-- 告警记录：保留1年
SELECT add_retention_policy('alarm_records', INTERVAL '1 year', if_not_exists => TRUE);

-- =====================================================
-- 2. 连续聚合自动刷新策略 (Continuous Aggregate Refresh Policies)
-- =====================================================

-- 小时级聚合：每30分钟刷新一次，刷新过去2小时的数据
SELECT add_continuous_aggregate_policy('sensor_data_hourly',
    start_offset => INTERVAL '2 hours',
    end_offset   => INTERVAL '1 minute',
    schedule_interval => INTERVAL '30 minutes',
    if_not_exists => TRUE
);

-- 日级聚合：每天刷新一次，刷新过去3天的数据
SELECT add_continuous_aggregate_policy('sensor_data_daily',
    start_offset => INTERVAL '3 days',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- =====================================================
-- 3. 用户自定义动作：压缩策略 (User-Defined Actions - Compression)
-- =====================================================

-- 启用传感器数据表的压缩
ALTER TABLE sensor_data SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'sensor_id',
    timescaledb.compress_orderby = 'time DESC'
);

-- 添加压缩策略：超过2天的数据自动压缩
SELECT add_compression_policy('sensor_data', INTERVAL '2 days', if_not_exists => TRUE);

-- =====================================================
-- 4. 信息视图：查看策略状态
-- =====================================================

CREATE OR REPLACE VIEW timescaledb_policy_status AS
SELECT
    j.job_id,
    j.proc_name,
    j.proc_schema,
    j.hypertable_schema,
    j.hypertable_name,
    j.config,
    js.next_start,
    js.last_run_status,
    js.last_finish,
    js.total_runs,
    js.total_successes,
    js.total_failures
FROM timescaledb_information.jobs j
LEFT JOIN timescaledb_information.job_stats js ON j.job_id = js.job_id
ORDER BY j.hypertable_name, j.proc_name;

-- =====================================================
-- 5. 辅助函数
-- =====================================================

-- 查看当前数据库大小
CREATE OR REPLACE FUNCTION get_db_size()
RETURNS TEXT AS $$
DECLARE
    size_text TEXT;
BEGIN
    SELECT pg_size_pretty(pg_database_size(current_database())) INTO size_text;
    RETURN size_text;
END;
$$ LANGUAGE plpgsql;

-- 查看hypertable大小
CREATE OR REPLACE FUNCTION get_hypertable_sizes()
RETURNS TABLE (
    hypertable_name TEXT,
    table_size TEXT,
    index_size TEXT,
    total_size TEXT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        h.hypertable_name::TEXT,
        pg_size_pretty(ht.table_size)::TEXT,
        pg_size_pretty(ht.index_size)::TEXT,
        pg_size_pretty(ht.total_size)::TEXT
    FROM timescaledb_information.hypertables h
    JOIN LATERAL hypertable_detailed_size(h.hypertable_name::regclass, h.hypertable_schema)
         AS ht(table_size bigint, index_size bigint, toast_size bigint, total_size bigint)
         ON true
    ORDER BY ht.total_size DESC;
END;
$$ LANGUAGE plpgsql;
