package seepage_simulator

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"tashan-weir-seepage/internal/database"
	"tashan-weir-seepage/internal/message"
	"tashan-weir-seepage/internal/metrics"
	"tashan-weir-seepage/internal/simulation"
)

type HydraulicsConfig struct {
	DamGeometry simulation.DamGeometry `json:"dam_geometry"`
	Hydrology   struct {
		DefaultUpstreamWL   float64 `json:"default_upstream_wl"`
		DefaultDownstreamWL float64 `json:"default_downstream_wl"`
		WaterDensity        float64 `json:"water_density"`
		Gravity             float64 `json:"gravity"`
	} `json:"hydrology"`
	Seepage struct {
		BasePermeability          float64 `json:"base_permeability"`
		FoundationPermeabilityRatio float64 `json:"foundation_permeability_ratio"`
		InterfaceEnabled          bool    `json:"interface_enabled"`
		InterfaceThicknessRatio   float64 `json:"interface_thickness_ratio"`
		InterfacePermeabilityRatio float64 `json:"interface_permeability_ratio"`
		BlanketPermeabilityRatio  float64 `json:"blanket_permeability_ratio"`
		GridNX                    int     `json:"grid_nx"`
		GridNY                    int     `json:"grid_ny"`
		SolverTolerance           float64 `json:"solver_tolerance"`
		SolverMaxIter             int     `json:"solver_max_iter"`
		SorOmega                  float64 `json:"sor_omega"`
	} `json:"seepage"`
}

type SeepageSimulator struct {
	cfg    HydraulicsConfig
	store  *database.DataStore
	bus    *message.Bus
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
}

func LoadConfig(path string) (HydraulicsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return HydraulicsConfig{}, err
	}
	var c HydraulicsConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return HydraulicsConfig{}, err
	}
	return c, nil
}

func New(cfg HydraulicsConfig, store *database.DataStore, bus *message.Bus) *SeepageSimulator {
	ctx, cancel := context.WithCancel(context.Background())
	return &SeepageSimulator{
		cfg:    cfg,
		store:  store,
		bus:    bus,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *SeepageSimulator) Start() {
	s.wg.Add(1)
	go s.loop()
	log.Println("[seepage_simulator] started")
}

func (s *SeepageSimulator) Stop() {
	s.cancel()
	s.wg.Wait()
	log.Println("[seepage_simulator] stopped")
}

func (s *SeepageSimulator) loop() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case req, ok := <-s.bus.SimRequestCh:
			if !ok {
				return
			}
			res := s.Run(req)
			if req.ResponseCh != nil {
				select {
				case req.ResponseCh <- res:
				default:
					log.Printf("[seepage_simulator] response channel full for %s", req.RequestID)
				}
			}
		}
	}
}

func (s *SeepageSimulator) Run(req message.SimRequestMsg) *message.SimResultMsg {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	metrics.IncSeepageSimulationRequests()
	upWL := req.UpstreamWL
	if upWL <= 0 {
		upWL = s.cfg.Hydrology.DefaultUpstreamWL
	}
	downWL := req.DownstreamWL
	if downWL <= 0 {
		downWL = s.cfg.Hydrology.DefaultDownstreamWL
	}

	permeability := s.cfg.Seepage.BasePermeability
	if req.Permeability > 0 {
		permeability = req.Permeability
	}

	solver := simulation.NewSeepageSolverWithConfig(
		s.cfg.DamGeometry,
		permeability,
		s.cfg.Seepage.GridNX,
		s.cfg.Seepage.GridNY,
		s.cfg.Seepage.FoundationPermeabilityRatio,
		s.cfg.Seepage.InterfaceEnabled,
		s.cfg.Seepage.InterfaceThicknessRatio,
		s.cfg.Seepage.InterfacePermeabilityRatio,
	)

	blanketLen := req.BlanketLength
	blanketThick := req.BlanketThickness
	if blanketLen > 0 && blanketThick > 0 {
		solver.SetBlanket(blanketLen, blanketThick, permeability*s.cfg.Seepage.BlanketPermeabilityRatio)
	}

	sim, grids, err := solver.Run(upWL, downWL, req.RequestID)
	elapsed := time.Since(start)
	metrics.ObserveSeepageSimulationDuration(elapsed)
	elapsedMs := elapsed.Milliseconds()

	if err != nil {
		return &message.SimResultMsg{
			RequestID: req.RequestID,
			Success:   false,
			Error:     err.Error(),
		}
	}

	sim.CalculationTimeMs = elapsedMs
	sim.Parameters["solver"] = "FDM+InterfaceElement"
	sim.Parameters["grid_nx"] = s.cfg.Seepage.GridNX
	sim.Parameters["grid_ny"] = s.cfg.Seepage.GridNY

	if s.store != nil {
		ctxDB, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		id, dbErr := s.store.InsertSimulation(ctxDB, sim)
		if dbErr == nil {
			sim.ID = id
			if len(grids) > 0 {
				_ = s.store.InsertSimulationGrids(ctxDB, id, grids)
			}
		} else {
			log.Printf("[seepage_simulator] db insert failed: %v", dbErr)
		}
	}

	return &message.SimResultMsg{
		RequestID:       req.RequestID,
		Success:         true,
		Simulation:      sim,
		Grids:           grids,
		SeepageFlow:     sim.TotalSeepageFlow,
		MaxPorePressure: sim.MaxPorePressure,
	}
}
