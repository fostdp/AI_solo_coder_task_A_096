# 古代它山堰坝体结构渗流仿真与防渗优化系统

> Tashan Weir Seepage Simulation and Anti-seepage Optimization System

基于 Go + Three.js + TimescaleDB + MQTT 的古代它山堰坝体渗流仿真、防渗优化和实时监测系统。

---

## 目录

- [系统架构](#系统架构)
- [模块说明](#模块说明)
- [快速开始](#快速开始)
- [传感器模拟器](#传感器模拟器)
- [监控与运维](#监控与运维)
- [配置参考](#配置参考)
- [API 接口](#api-接口)

---

## 系统架构

```
                          ┌──────────────────────────────────────────────────────────┐
                          │                     前端 (Frontend)                       │
                          │  ┌───────────────────┐   ┌────────────────────────────┐  │
                          │  │  tuoshan_dam_3d.js│   │     seepage_panel.js       │  │
                          │  │  (Three.js 3D可视化)│   │  (控制面板/数据展示)       │  │
                          │  └───────────────────┘   └────────────────────────────┘  │
                          │                       Gzip 压缩                           │
                          └──────────────────────────┬───────────────────────────────┘
                                                     │ HTTP/REST + WebSocket
                          ┌──────────────────────────▼───────────────────────────────┐
                          │                    Go 后端服务 (Backend)                    │
                          │  ┌─────────────────────────────────────────────────────┐  │
                          │  │                    Gin API Server                    │  │
                          │  │   ┌──────────────┐  ┌──────────────┐  ┌──────────┐  │  │
                          │  │   │ pprof (:6060) │  │ /metrics     │  │ Gzip 中间│  │  │
                          │  │   └──────────────┘  └──────────────┘  └──────────┘  │  │
                          │  └─────────────────────────────────────────────────────┘  │
                          │  ┌──────────────┐ ┌──────────────┐ ┌──────────────────┐  │
                          │  │ dtu_receiver │ │  alarm_mqtt  │ │seepage_simulator │  │
                          │  │  (传感器采集) │ │  (告警推送)  │ │  (有限元渗流)     │  │
                          │  └───────┬──────┘ └──────┬───────┘ └────────┬─────────┘  │
                          │          │        Channel    │  Message Bus    │            │
                          │          └────────────────┼─────────────────┘            │
                          │                           │                              │
                          │              ┌────────────▼────────────┐                 │
                          │              │ anti_seepage_optimizer  │                 │
                          │              │  (NSGA-II多目标优化)      │                 │
                          │              └─────────────────────────┘                 │
                          └──────┬──────────────────┬──────────────────┬───────────────┘
                                 │                  │                  │
                          ┌──────▼──────┐   ┌──────▼──────┐   ┌──────▼───────┐
                          │ TimescaleDB │   │  MQTT Broker│   │   Prometheus │
                          │ (时序数据库) │   │ (Mosquitto) │   │  (指标采集)   │
                          └─────────────┘   └─────────────┘   └──────┬───────┘
                                                                      │
                                                               ┌──────▼───────┐
                                                               │   Grafana    │
                                                               │  (可视化)    │
                                                               └──────────────┘
                          ┌──────────────────────────────────────────────────────────┐
                          │                Sensor Simulator (传感器模拟器)            │
                          │   ┌─────────────┐ ┌─────────────┐ ┌──────────────────┐   │
                          │   │ 水位控制     │ │渗透系数控制 │ │ HTTP API :8081   │   │
                          │   └─────────────┘ └─────────────┘ └──────────────────┘   │
                          └──────────────────────────────────────────────────────────┘
```

### 架构特点

- **模块化后端**：4个独立Go模块（dtu_receiver, seepage_simulator, anti_seepage_optimizer, alarm_mqtt）通过Channel异步通信
- **配置外置**：水力学参数和遗传算子通过JSON配置文件管理
- **时序数据库**：TimescaleDB存储传感器时序数据，支持自动降采样和数据保留策略
- **实时告警**：MQTT协议推送告警消息
- **可观测性**：Prometheus指标采集 + pprof性能分析 + Grafana可视化
- **前端优化**：静态资源Gzip压缩 + 模块化JS设计

---

## 模块说明

### 后端模块

| 模块 | 路径 | 职责 |
|------|------|------|
| **dtu_receiver** | `backend/internal/dtu_receiver/` | 传感器数据采集、校验、存储 |
| **seepage_simulator** | `backend/internal/seepage_simulator/` | 有限差分法(FDM)渗流场计算 |
| **anti_seepage_optimizer** | `backend/internal/anti_seepage_optimizer/` | NSGA-II多目标遗传算法，防渗铺盖优化 |
| **alarm_mqtt** | `backend/internal/alarm_mqtt/` | 阈值评估、冷却检查、MQTT告警推送 |
| **metrics** | `backend/internal/metrics/` | Prometheus指标采集和Gin中间件 |
| **message** | `backend/internal/message/` | 模块间Channel消息总线 |

### 前端模块

| 文件 | 路径 | 职责 |
|------|------|------|
| **tuoshan_dam_3d.js** | `frontend/js/tuoshan_dam_3d.js` | Three.js 3D大坝结构和渗流场可视化 |
| **seepage_panel.js** | `frontend/js/seepage_panel.js` | IIFE封装的控制面板、图表、数据展示 |
| **app.js** | `frontend/js/app.js` | 初始化和模块协调（77行精简版） |

---

## 快速开始

### 环境要求

- Docker 20.10+
- Docker Compose v2+
- 至少 4GB 可用内存

### 一键部署

```bash
# 1. 克隆项目
git clone <repository-url>
cd AI_solo_coder_task_A_096

# 2. 启动所有服务
docker compose up -d

# 3. 查看服务状态
docker compose ps

# 4. 查看日志
docker compose logs -f backend
docker compose logs -f sensor-simulator
```

### 服务访问地址

启动成功后，可通过以下地址访问：

| 服务 | 地址 | 默认端口 | 说明 |
|------|------|----------|------|
| **Web前端** | http://localhost:8080/ | 8080 | 3D可视化 + 控制面板 |
| **后端API** | http://localhost:8080/api/v1/health | 8080 | REST API |
| **Prometheus指标** | http://localhost:8080/metrics | 8080 | Prometheus格式指标 |
| **pprof性能分析** | http://localhost:6060/debug/pprof/ | 6060 | Go性能分析 |
| **模拟器API** | http://localhost:8081/health | 8081 | 传感器模拟器控制 |
| **Prometheus UI** | http://localhost:9090/ | 9090 | 指标查询 |
| **Grafana** | http://localhost:3000/ | 3000 | 可视化仪表盘 (admin/admin123) |
| **MQTT Broker** | mqtt://localhost:1883 | 1883 | MQTT消息推送 |
| **TimescaleDB** | postgresql://localhost:5432/tashan_weir | 5432 | 数据库 (postgres/postgres) |

### 停止与清理

```bash
# 停止服务
docker compose down

# 停止并删除数据卷（⚠️ 会删除所有数据）
docker compose down -v
```

### 本地开发模式

```bash
# 后端编译
cd backend
go build -o bin/server ./cmd/main.go

# 后端运行（需要先启动TimescaleDB和MQTT）
./bin/server

# 模拟器编译
cd ../simulator
go build -o bin/simulator ./sensor_simulator.go
./bin/simulator
```

---

## 传感器模拟器

### 功能特性

坝体传感器模拟器可以模拟真实DTU设备发送传感器数据，支持：

- ✅ **15个传感器**：扬压力计、渗流量、水位计、冲刷深度、浸润线
- ✅ **实时水位控制**：动态设置上下游水位
- ✅ **渗透系数调节**：模拟不同基岩渗透性
- ✅ **告警触发测试**：手动触发告警事件
- ✅ **HTTP API控制**：运行时动态调整参数
- ✅ **可重复仿真**：固定随机种子实现可重复测试

### 环境变量配置

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| `API_URL` | `http://backend:8080/api/v1` | 后端API地址 |
| `DTU_ID` | `DTU-TASHAN-001` | DTU设备ID |
| `INTERVAL_SECONDS` | `600` | 采样间隔（秒） |
| `SIM_SPEED` | `1` | 仿真速度倍率 |
| `UPSTREAM_WL` | `-1` | 上游水位（米），-1表示不覆盖默认值 |
| `DOWNSTREAM_WL` | `-1` | 下游水位（米），-1表示不覆盖默认值 |
| `FOUNDATION_PERMEABILITY` | `-1` | 基岩渗透系数，-1表示不覆盖默认值 |
| `RANDOM_SEED` | 时间戳 | 随机种子，设置后可重复仿真 |
| `ENABLE_HTTP_API` | `true` | 是否启用HTTP控制API |
| `HTTP_PORT` | `8081` | HTTP API监听端口 |

### HTTP API 接口

#### 1. 健康检查
```bash
curl http://localhost:8081/health
```

响应：
```json
{
  "status": "ok",
  "uptime": "1h23m45s",
  "cycles_sent": 42
}
```

#### 2. 获取当前配置
```bash
curl http://localhost:8081/config
```

响应：
```json
{
  "upstream_water_level": 3.5,
  "downstream_water_level": 0.5,
  "foundation_permeability": 1.0e-7,
  "sim_speed": 10,
  "interval_seconds": 60
}
```

#### 3. 更新配置
```bash
curl -X POST http://localhost:8081/config \
  -H "Content-Type: application/json" \
  -d '{
    "upstream_water_level": 8.5,
    "downstream_water_level": 3.2,
    "foundation_permeability": 5.0e-7,
    "sim_speed": 20
  }'
```

参数说明：
- `upstream_water_level`: 上游水位（米），影响扬压力和渗流量
- `downstream_water_level`: 下游水位（米）
- `foundation_permeability`: 基岩渗透系数，值越大渗流量越大
- `sim_speed`: 仿真速度倍率
- `interval_seconds`: 采样间隔（秒）

#### 4. 获取传感器实时状态
```bash
curl http://localhost:8081/status
```

响应：
```json
{
  "timestamp": "2026-06-17T12:34:56Z",
  "sensors": {
    "WL-001": 8.52,
    "WL-002": 3.18,
    "SM-001": 18.45,
    "PZ-003": 68.7
  }
}
```

#### 5. 手动触发告警
```bash
curl -X POST http://localhost:8081/trigger-alarm \
  -H "Content-Type: application/json" \
  -d '{
    "sensor_id": "PZ-001",
    "level": "danger"
  }'
```

参数：
- `sensor_id`: 传感器ID（如 PZ-001, SM-001）
- `level`: `warning` 或 `danger`

### 使用示例

```bash
# 模拟汛期高水位
curl -X POST http://localhost:8081/config \
  -H "Content-Type: application/json" \
  -d '{"upstream_water_level": 9.0, "sim_speed": 60}'

# 模拟地基恶化（渗透系数增大10倍）
curl -X POST http://localhost:8081/config \
  -H "Content-Type: application/json" \
  -d '{"foundation_permeability": 1.0e-6}'

# 触发测试告警
curl -X POST http://localhost:8081/trigger-alarm \
  -H "Content-Type: application/json" \
  -d '{"sensor_id": "SM-001", "level": "danger"}'
```

---

## 监控与运维

### Prometheus 指标

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `http_requests_total` | Counter | HTTP请求总数（标签: method, path, status_code） |
| `http_request_duration_seconds` | Histogram | HTTP请求延迟分布 |
| `sensor_data_received_total` | Counter | 接收的传感器数据总数 |
| `simulation_requests_total` | Counter | 渗流仿真请求总数 |
| `simulation_duration_seconds` | Histogram | 渗流仿真耗时 |
| `optimization_requests_total` | Counter | 优化算法请求总数 |
| `optimization_duration_seconds` | Histogram | 优化算法耗时 |
| `alarms_triggered_total` | Counter | 告警触发总数（标签: level） |
| `mqtt_published_total` | Counter | MQTT消息发布总数 |
| `go_goroutines` | Gauge | Goroutine数量 |
| `go_memstats_alloc_bytes` | Gauge | 内存分配大小 |

### pprof 性能分析

```bash
# 查看内存使用
go tool pprof http://localhost:6060/debug/pprof/heap

# 查看CPU使用（30秒采样）
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# 查看Goroutine
go tool pprof http://localhost:6060/debug/pprof/goroutine

# 生成火焰图（需要graphviz）
go tool pprof -http=:8082 http://localhost:6060/debug/pprof/profile?seconds=30
```

### TimescaleDB 数据保留策略

| 数据类型 | 保留时间 | 说明 |
|----------|----------|------|
| 原始传感器数据 | 7天 | 高频原始数据自动清理 |
| 小时级聚合 | 90天 | AVG/MAX/MIN/LAST |
| 日级聚合 | 2年 | 长期趋势分析 |
| 告警记录 | 1年 | 告警历史 |
| 超过2天的原始数据 | 自动压缩 | TimescaleDB原生压缩 |

查看策略状态：
```sql
-- 连接数据库
docker compose exec timescaledb psql -U postgres -d tashan_weir

-- 查看所有后台任务
SELECT * FROM timescaledb_policy_status;

-- 查看数据库大小
SELECT get_db_size();

-- 查看hypertable大小
SELECT * FROM get_hypertable_sizes();
```

### 常用运维命令

```bash
# 查看所有服务状态
docker compose ps

# 查看某个服务日志
docker compose logs -f --tail=100 backend
docker compose logs -f --tail=100 sensor-simulator

# 重启某个服务
docker compose restart backend

# 重新构建并启动
docker compose up -d --build backend sensor-simulator

# 进入数据库
docker compose exec timescaledb psql -U postgres -d tashan_weir

# 订阅MQTT告警消息
docker compose exec mqtt-broker mosquitto_sub -v -t 'tashan-weir/#'
```

---

## 配置参考

### 水力学参数 (backend/configs/hydraulics.json)

```json
{
  "dam_geometry": {
    "length": 113.7,
    "height": 3.85,
    "top_width": 4.8,
    "upstream_slope": 0.35,
    "downstream_slope": 0.6,
    "foundation_depth": 5.0
  },
  "hydrology": {
    "default_upstream_wl": 3.5,
    "default_downstream_wl": 0.5
  },
  "seepage": {
    "base_permeability": 1.0e-7,
    "grid_nx": 200,
    "grid_ny": 80
  }
}
```

### 遗传算法参数 (backend/configs/genetic_algo.json)

```json
{
  "algorithm": "NSGA-II",
  "population_size": 60,
  "generations": 80,
  "decision_variables": {
    "blanket_length": { "min": 1.0, "max": 20.0 },
    "blanket_thickness": { "min": 0.2, "max": 3.0 }
  },
  "operators": {
    "sbx_eta_c": 15.0,
    "sbx_crossover_prob": 0.9,
    "mutation_prob": 0.1
  }
}
```

---

## API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/health` | 健康检查 |
| GET | `/api/v1/configs` | 获取系统配置 |
| GET | `/api/v1/dam-info` | 获取坝体基本信息 |
| GET | `/api/v1/sensors` | 获取传感器配置列表 |
| GET | `/api/v1/sensors/latest` | 获取传感器最新值 |
| GET | `/api/v1/sensors/:id/data?hours=24` | 获取传感器历史数据 |
| POST | `/api/v1/dtu/data` | DTU数据上报 |
| GET | `/api/v1/simulations` | 获取仿真记录列表 |
| POST | `/api/v1/simulations/run` | 运行渗流仿真 |
| GET | `/api/v1/simulations/:id` | 获取仿真详情 |
| GET | `/api/v1/simulations/:id/grids` | 获取仿真网格数据 |
| GET | `/api/v1/optimizations` | 获取优化记录列表 |
| POST | `/api/v1/optimizations/run` | 运行防渗优化 |
| GET | `/api/v1/alarms` | 获取告警记录 |
| PUT | `/api/v1/alarms/:id/handle` | 确认处理告警 |
| GET | `/metrics` | Prometheus指标 |
| GET | `/debug/pprof/*` | pprof性能分析 |

### 渗流仿真请求示例

```bash
curl -X POST http://localhost:8080/api/v1/simulations/run \
  -H "Content-Type: application/json" \
  -d '{
    "simulation_name": "汛期高水位仿真",
    "upstream_water_level": 8.5,
    "downstream_water_level": 3.2,
    "grid_resolution_x": 100,
    "grid_resolution_y": 40,
    "blanket_length": 15.0,
    "blanket_thickness": 1.5
  }'
```

### 防渗优化请求示例

```bash
curl -X POST http://localhost:8080/api/v1/optimizations/run \
  -H "Content-Type: application/json" \
  -d '{
    "optimization_name": "汛期方案优化",
    "upstream_water_level": 8.5,
    "downstream_water_level": 3.2,
    "population_size": 50,
    "max_generations": 50
  }'
```

---

## 许可证

本项目仅供学术研究和工程参考使用。
