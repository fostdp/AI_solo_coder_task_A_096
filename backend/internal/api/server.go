package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"tashan-weir-seepage/internal/alarm_mqtt"
	"tashan-weir-seepage/internal/anti_seepage_optimizer"
	"tashan-weir-seepage/internal/database"
	"tashan-weir-seepage/internal/dtu_receiver"
	"tashan-weir-seepage/internal/message"
	"tashan-weir-seepage/internal/models"
	"tashan-weir-seepage/internal/seepage_simulator"
)

const (
	hydraulicsCfgDefault = "configs/hydraulics.json"
	geneticCfgDefault    = "configs/genetic_algo.json"
)

type Server struct {
	router       *gin.Engine
	store        *database.DataStore
	bus          *message.Bus
	alarmSvc     *alarm_mqtt.AlarmMQTT
	dtu          *dtu_receiver.DTUReceiver
	simulator    *seepage_simulator.SeepageSimulator
	optimizer    *anti_seepage_optimizer.AntiSeepageOptimizer
	hydraCfg     seepage_simulator.HydraulicsConfig
	genCfg       anti_seepage_optimizer.GeneticConfig
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewServer(store *database.DataStore) *Server {
	hydraCfg, err := seepage_simulator.LoadConfig(resolvePath(hydraulicsCfgDefault))
	if err != nil {
		log.Printf("[server] hydraulics config load failed (%v), using defaults", err)
		hydraCfg = defaultHydraulics()
	}
	genCfg, err := anti_seepage_optimizer.LoadConfig(resolvePath(geneticCfgDefault))
	if err != nil {
		log.Printf("[server] genetic config load failed (%v), using defaults", err)
		genCfg = defaultGenetic()
	}

	ctx, cancel := context.WithCancel(context.Background())
	bus := message.NewBus(128)

	alarmSvc := alarm_mqtt.New(store, bus)
	if mqttErr := alarmSvc.ConnectMQTT(); mqttErr != nil {
		log.Printf("[server] MQTT unavailable: %v (continuing without push)", mqttErr)
	}

	s := &Server{
		router:    gin.Default(),
		store:     store,
		bus:       bus,
		alarmSvc:  alarmSvc,
		dtu:       dtu_receiver.New(store, bus),
		simulator: seepage_simulator.New(hydraCfg, store, bus),
		optimizer: anti_seepage_optimizer.New(genCfg, resolvePath(hydraulicsCfgDefault), store, bus),
		hydraCfg:  hydraCfg,
		genCfg:    genCfg,
		ctx:       ctx,
		cancel:    cancel,
	}

	s.setupCORS()
	s.setupRoutes()
	s.loadSensorConfigs()
	s.startModules()
	return s
}

func resolvePath(p string) string {
	if _, err := os.Stat(p); err == nil {
		return p
	}
	alt := "../" + p
	if _, err := os.Stat(alt); err == nil {
		return alt
	}
	return p
}

func defaultHydraulics() seepage_simulator.HydraulicsConfig {
	var c seepage_simulator.HydraulicsConfig
	c.DamGeometry.Length = 113.7
	c.DamGeometry.Height = 3.85
	c.DamGeometry.TopWidth = 4.8
	c.DamGeometry.UpstreamSlope = 0.35
	c.DamGeometry.DownstreamSlope = 0.6
	c.DamGeometry.FoundationDepth = 5.0
	c.Hydrology.DefaultUpstreamWL = 3.5
	c.Hydrology.DefaultDownstreamWL = 0.5
	c.Hydrology.WaterDensity = 1000
	c.Hydrology.Gravity = 9.81
	c.Seepage.BasePermeability = 1e-7
	c.Seepage.FoundationPermeabilityRatio = 5.0
	c.Seepage.InterfaceEnabled = true
	c.Seepage.InterfaceThicknessRatio = 2.0
	c.Seepage.InterfacePermeabilityRatio = 0.5
	c.Seepage.BlanketPermeabilityRatio = 0.05
	c.Seepage.GridNX = 200
	c.Seepage.GridNY = 80
	c.Seepage.SolverTolerance = 1e-6
	c.Seepage.SolverMaxIter = 5000
	c.Seepage.SorOmega = 1.5
	return c
}

func defaultGenetic() anti_seepage_optimizer.GeneticConfig {
	var c anti_seepage_optimizer.GeneticConfig
	c.Algorithm = "NSGA-II"
	c.PopulationSize = 60
	c.Generations = 80
	c.DecisionVariables.BlanketLength.Min = 1.0
	c.DecisionVariables.BlanketLength.Max = 20.0
	c.DecisionVariables.BlanketThickness.Min = 0.2
	c.DecisionVariables.BlanketThickness.Max = 3.0
	c.Operators.SBXEtaC = 15
	c.Operators.SBXCrossoverProb = 0.9
	c.Operators.PolynomialEtaM = 20
	c.Operators.MutationProb = 0.1
	c.Operators.MutationPert = 0.1
	c.Operators.TournamentSize = 2
	c.CostConfig.ConcreteUnitPrice = 350
	c.CostConfig.ClayUnitPrice = 120
	c.CostConfig.GeomembraneUnitPrice = 85
	c.CostConfig.ExcavationUnitPrice = 45
	c.CostConfig.MaxBudget = 500000
	c.Parallel.Enabled = true
	c.Parallel.MaxWorkers = 4
	return c
}

func (s *Server) startModules() {
	s.dtu.Start()
	s.alarmSvc.Start()
	s.simulator.Start()
	s.optimizer.Start()
	log.Println("[server] all modules started")
}

func (s *Server) Stop() {
	s.cancel()
	s.bus.Close()
	s.dtu.Stop()
	s.alarmSvc.Stop()
	s.simulator.Stop()
	s.optimizer.Stop()
	log.Println("[server] stopped")
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
	api.GET("/configs", s.handleGetConfigs)

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
		log.Printf("[server] failed to load sensor configs: %v", err)
		return
	}
	s.alarmSvc.UpdateSensorConfigs(configs)
	log.Printf("[server] loaded %d sensor configurations", len(configs))
}

func (s *Server) Router() *gin.Engine { return s.router }

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now(),
		"service":   "tashan-weir-seepage-backend",
		"modules": gin.H{
			"dtu_receiver":            "running",
			"alarm_mqtt":              "running",
			"seepage_simulator":       "running",
			"anti_seepage_optimizer":  "running",
		},
	})
}

func (s *Server) handleGetConfigs(c *gin.Context) {
	hydraJSON, _ := json.Marshal(s.hydraCfg)
	genJSON, _ := json.Marshal(s.genCfg)
	var hydra, gen map[string]interface{}
	_ = json.Unmarshal(hydraJSON, &hydra)
	_ = json.Unmarshal(genJSON, &gen)
	c.JSON(http.StatusOK, gin.H{"hydraulics": hydra, "genetic_algo": gen})
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

	inserted, err := s.dtu.HandleAndStore(ctx, &payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "DTU validation failed: " + err.Error()})
		return
	}

	alarms := s.collectRecentAlarms()

	if s.alarmSvc != nil {
		go s.alarmSvc.PublishSensorData(ctx, payload.DTUID, payload.Sensors)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"inserted":     inserted,
		"alarms_count": len(alarms),
		"alarms":       alarms,
		"dtu_id":       payload.DTUID,
	})
}

func (s *Server) collectRecentAlarms() []models.AlarmRecord {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	alarms, _ := s.store.GetRecentAlarms(ctx, 10, true)
	return alarms
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
	c.JSON(http.StatusOK, gin.H{"simulation_id": id, "count": len(grids), "grids": grids})
}

func (s *Server) handleRunSimulation(c *gin.Context) {
	var req models.SimulationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.UpstreamWaterLevel <= 0 {
		req.UpstreamWaterLevel = s.hydraCfg.Hydrology.DefaultUpstreamWL
	}
	if req.DownstreamWaterLevel <= 0 {
		req.DownstreamWaterLevel = s.hydraCfg.Hydrology.DefaultDownstreamWL
	}
	if req.SimulationName == "" {
		req.SimulationName = "Sim_" + time.Now().Format("20060102_150405")
	}

	respCh := make(chan *message.SimResultMsg, 1)
	reqID := fmt.Sprintf("sim_%d", time.Now().UnixNano())

	msg := message.SimRequestMsg{
		RequestID:        reqID,
		UpstreamWL:       req.UpstreamWaterLevel,
		DownstreamWL:     req.DownstreamWaterLevel,
		Permeability:     req.PermeabilityK,
		ResponseCh:       respCh,
	}
	if req.BlanketLength != nil {
		msg.BlanketLength = *req.BlanketLength
	}
	if req.BlanketThickness != nil {
		msg.BlanketThickness = *req.BlanketThickness
	}

	select {
	case s.bus.SimRequestCh <- msg:
	case <-time.After(3 * time.Second):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "simulator busy"})
		return
	}

	select {
	case res := <-respCh:
		if !res.Success {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "simulation failed: " + res.Error})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"simulation": res.Simulation,
			"grid_count": len(res.Grids),
			"grids":      res.Grids,
		})
	case <-time.After(60 * time.Second):
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "simulation timeout"})
	}
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
	var req models.OptimizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.UpstreamWaterLevel <= 0 {
		damInfo, _ := s.store.GetDamInfo(c.Request.Context())
		if damInfo != nil {
			req.UpstreamWaterLevel = damInfo.DesignUpstreamWaterLevel
			req.DownstreamWaterLevel = damInfo.DesignDownstreamWaterLevel
		} else {
			req.UpstreamWaterLevel = s.hydraCfg.Hydrology.DefaultUpstreamWL
			req.DownstreamWaterLevel = s.hydraCfg.Hydrology.DefaultDownstreamWL
		}
	}
	if req.OptimizationName == "" {
		req.OptimizationName = "Opt_" + time.Now().Format("20060102_150405")
	}

	respCh := make(chan *message.OptResultMsg, 1)
	reqID := fmt.Sprintf("opt_%d", time.Now().UnixNano())
	msg := message.OptRequestMsg{
		RequestID:    reqID,
		UpstreamWL:   req.UpstreamWaterLevel,
		DownstreamWL: req.DownstreamWaterLevel,
		ResponseCh:   respCh,
	}

	select {
	case s.bus.OptRequestCh <- msg:
	case <-time.After(3 * time.Second):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "optimizer busy"})
		return
	}

	select {
	case res := <-respCh:
		if !res.Success {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "optimization failed: " + res.Error})
			return
		}
		summary := gin.H{}
		if res.Result != nil {
			summary = gin.H{
				"baseline_flow_lps":       res.Result.BaselineSeepageFlow * 1000,
				"optimized_flow_lps":      res.Result.OptimizedSeepageFlow * 1000,
				"reduction_rate":          res.Result.FlowReductionRate,
				"best_blanket_length":     res.Result.BlanketLength,
				"best_blanket_thickness":  res.Result.BlanketThickness,
				"elapsed_ms":              res.ElapsedMs,
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"optimization": res.Result,
			"pareto_front": res.ParetoFront,
			"summary":      summary,
		})
	case <-time.After(300 * time.Second):
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "optimization timeout"})
	}
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
	c.JSON(http.StatusOK, gin.H{"count": len(alarms), "alarms": alarms})
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
	_ = c.ShouldBindJSON(&body)
	if err := s.store.AcknowledgeAlarm(ctx, id, body.HandledBy, body.HandleNote); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "acknowledged", "alarm_id": id})
}
