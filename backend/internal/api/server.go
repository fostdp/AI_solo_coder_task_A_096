package api

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"tashan-weir-seepage/internal/database"
	"tashan-weir-seepage/internal/models"
	"tashan-weir-seepage/internal/mqtt"
	"tashan-weir-seepage/internal/optimization"
	"tashan-weir-seepage/internal/simulation"
)

type Server struct {
	router          *gin.Engine
	store           *database.DataStore
	mqttSvc         *mqtt.MQTTService
	alarmChecker    *mqtt.AlarmChecker
	solver          *simulation.SeepageSolver
	optimizer       *optimization.GeneticOptimizer
}

func NewServer(store *database.DataStore, mqttSvc *mqtt.MQTTService) *Server {
	geo := simulation.DamGeometry{
		Length:          113.7,
		Height:          3.85,
		TopWidth:        4.8,
		UpstreamSlope:   0.35,
		DownstreamSlope: 0.6,
		FoundationDepth: 5.0,
	}

	basePermeability := 1e-7

	s := &Server{
		router:       gin.Default(),
		store:        store,
		mqttSvc:      mqttSvc,
		alarmChecker: mqtt.NewAlarmChecker(),
		solver:       simulation.NewSeepageSolver(geo, basePermeability),
		optimizer:    optimization.NewGeneticOptimizer(geo, basePermeability),
	}

	s.setupCORS()
	s.setupRoutes()
	s.loadSensorConfigs()
	return s
}

func (s *Server) setupCORS() {
	s.router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})
}

func (s *Server) setupRoutes() {
	api := s.router.Group("/api/v1")

	api.GET("/health", s.handleHealth)
	api.GET("/dam-info", s.handleGetDamInfo)

	api.GET("/sensors", s.handleGetSensors)
	api.GET("/sensors/:id/data", s.handleGetSensorData)
	api.GET("/sensors/latest", s.handleGetLatestSensorValues)

	api.POST("/dtu/data", s.handleDTUDataUpload)

	api.GET("/simulations", s.handleGetSimulations)
	api.GET("/simulations/:id", s.handleGetSimulation)
	api.GET("/simulations/:id/grids", s.handleGetSimulationGrids)
	api.POST("/simulations/run", s.handleRunSimulation)

	api.GET("/optimizations", s.handleGetOptimizations)
	api.POST("/optimizations/run", s.handleRunOptimization)

	api.GET("/alarms", s.handleGetAlarms)
	api.PUT("/alarms/:id/handle", s.handleAcknowledgeAlarm)

	s.router.Static("/frontend", "./frontend")
	s.router.GET("/", func(c *gin.Context) {
		c.File("./frontend/index.html")
	})
}

func (s *Server) loadSensorConfigs() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	configs, err := s.store.GetAllSensorConfigs(ctx)
	if err != nil {
		log.Printf("Failed to load sensor configs: %v", err)
		return
	}
	s.alarmChecker.UpdateSensorConfigs(configs)
	log.Printf("Loaded %d sensor configurations", len(configs))
}

func (s *Server) Router() *gin.Engine {
	return s.router
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now(),
		"service":   "tashan-weir-seepage-backend",
	})
}

func (s *Server) handleGetDamInfo(c *gin.Context) {
	ctx := c.Request.Context()
	damInfo, err := s.store.GetDamInfo(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, damInfo)
}

func (s *Server) handleGetSensors(c *gin.Context) {
	ctx := c.Request.Context()
	sensors, err := s.store.GetAllSensorConfigs(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sensors)
}

func (s *Server) handleGetSensorData(c *gin.Context) {
	ctx := c.Request.Context()
	sensorID := c.Param("id")
	hoursStr := c.DefaultQuery("hours", "24")
	hours, err := strconv.Atoi(hoursStr)
	if err != nil {
		hours = 24
	}

	data, err := s.store.GetRecentSensorData(ctx, sensorID, hours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (s *Server) handleGetLatestSensorValues(c *gin.Context) {
	ctx := c.Request.Context()
	data, err := s.store.GetLatestSensorValues(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (s *Server) handleDTUDataUpload(c *gin.Context) {
	ctx := c.Request.Context()
	var payload models.DTUPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now()
	}

	for i := range payload.Sensors {
		if payload.Sensors[i].Time.IsZero() {
			payload.Sensors[i].Time = payload.Timestamp
		}
	}

	err := s.store.InsertSensorDataBatch(ctx, payload.Sensors)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert data: " + err.Error()})
		return
	}

	var generatedAlarms []*models.AlarmRecord
	for _, sd := range payload.Sensors {
		if alarm := s.alarmChecker.CheckSensor(sd); alarm != nil {
			alarmID, dbErr := s.store.InsertAlarm(ctx, alarm)
			if dbErr != nil {
				log.Printf("Failed to insert alarm: %v", dbErr)
				continue
			}
			alarm.ID = alarmID

			configs := s.alarmChecker.GetConfigs()
			cfg := configs[sd.SensorID]
			if s.mqttSvc != nil {
				pubErr := s.mqttSvc.PublishAlarm(ctx, alarm, &cfg)
				if pubErr != nil {
					log.Printf("Failed to publish MQTT alarm: %v", pubErr)
				} else {
					s.store.UpdateAlarmMQTTPublished(ctx, alarmID)
				}
			}
			generatedAlarms = append(generatedAlarms, alarm)
		}
	}

	if s.mqttSvc != nil {
		go s.mqttSvc.PublishSensorData(ctx, payload.DTUID, payload.Sensors)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"inserted":      len(payload.Sensors),
		"alarms_count":  len(generatedAlarms),
		"alarms":        generatedAlarms,
		"dtu_id":        payload.DTUID,
	})
}

func (s *Server) handleGetSimulations(c *gin.Context) {
	ctx := c.Request.Context()
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)

	sims, err := s.store.GetSimulations(ctx, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sims)
}

func (s *Server) handleGetSimulation(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	sim, err := s.store.GetSimulation(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sim)
}

func (s *Server) handleGetSimulationGrids(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	grids, err := s.store.GetSimulationGrids(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"simulation_id": id,
		"count":         len(grids),
		"grids":         grids,
	})
}

func (s *Server) handleRunSimulation(c *gin.Context) {
	ctx := c.Request.Context()
	var req models.SimulationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UpstreamWaterLevel <= 0 || req.DownstreamWaterLevel <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid water levels"})
		return
	}

	if req.SimulationName == "" {
		req.SimulationName = "Simulation_" + time.Now().Format("20060102_150405")
	}

	simResult, grids, err := s.solver.RunSimulation(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Simulation failed: " + err.Error()})
		return
	}

	simID, err := s.store.InsertSimulation(ctx, simResult)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save simulation: " + err.Error()})
		return
	}
	simResult.ID = simID

	if len(grids) > 0 {
		go s.store.InsertSimulationGrids(ctx, simID, grids)
	}

	c.JSON(http.StatusOK, gin.H{
		"simulation": simResult,
		"grid_count": len(grids),
		"grids":      grids,
	})
}

func (s *Server) handleGetOptimizations(c *gin.Context) {
	ctx := c.Request.Context()
	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)

	opts, err := s.store.GetOptimizationResults(ctx, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, opts)
}

func (s *Server) handleRunOptimization(c *gin.Context) {
	ctx := c.Request.Context()
	var req models.OptimizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UpstreamWaterLevel <= 0 {
		damInfo, _ := s.store.GetDamInfo(ctx)
		req.UpstreamWaterLevel = damInfo.DesignUpstreamWaterLevel
		req.DownstreamWaterLevel = damInfo.DesignDownstreamWaterLevel
	}

	if req.OptimizationName == "" {
		req.OptimizationName = "GA_Opt_" + time.Now().Format("20060102_150405")
	}

	optResult, err := s.optimizer.Optimize(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Optimization failed: " + err.Error()})
		return
	}

	optID, err := s.store.InsertOptimizationResult(ctx, optResult)
	if err != nil {
		log.Printf("Failed to save optimization result: %v", err)
	}
	optResult.ID = optID

	c.JSON(http.StatusOK, gin.H{
		"optimization": optResult,
		"summary": gin.H{
			"baseline_flow_lps":   optResult.BaselineSeepageFlow * 1000,
			"optimized_flow_lps":  optResult.OptimizedSeepageFlow * 1000,
			"reduction_rate":      optResult.FlowReductionRate,
			"best_blanket_length": optResult.BlanketLength,
			"best_blanket_thickness": optResult.BlanketThickness,
		},
	})
}

func (s *Server) handleGetAlarms(c *gin.Context) {
	ctx := c.Request.Context()
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	unhandledOnly := c.DefaultQuery("unhandled", "false") == "true"

	alarms, err := s.store.GetRecentAlarms(ctx, limit, unhandledOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"count":  len(alarms),
		"alarms": alarms,
	})
}

func (s *Server) handleAcknowledgeAlarm(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var body struct {
		HandledBy  string `json:"handled_by"`
		HandleNote string `json:"handle_note"`
	}
	c.ShouldBindJSON(&body)

	query := `UPDATE alarm_records SET is_handled = TRUE, handled_by = $1, 
		handled_time = NOW(), handle_note = $2 WHERE id = $3`
	_, dbErr := s.store.Pool().Exec(ctx, query, body.HandledBy, body.HandleNote, id)
	if dbErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": dbErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "acknowledged", "alarm_id": id})
}

func (ac *mqtt.AlarmChecker) GetConfigs() map[string]models.SensorConfig {
	return ac.sensorConfigs
}
