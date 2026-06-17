package anti_seepage_optimizer

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"tashan-weir-seepage/internal/database"
	"tashan-weir-seepage/internal/message"
	"tashan-weir-seepage/internal/models"
	"tashan-weir-seepage/internal/optimization"
)

type GeneticConfig struct {
	Algorithm          string  `json:"algorithm"`
	PopulationSize     int     `json:"population_size"`
	Generations        int     `json:"generations"`
	DecisionVariables  struct {
		BlanketLength   struct{ Min, Max float64 } `json:"blanket_length"`
		BlanketThickness struct{ Min, Max float64 } `json:"blanket_thickness"`
	} `json:"decision_variables"`
	Operators struct {
		SBXEtaC           float64 `json:"sbx_eta_c"`
		SBXCrossoverProb  float64 `json:"sbx_crossover_prob"`
		PolynomialEtaM    float64 `json:"polynomial_eta_m"`
		MutationProb      float64 `json:"mutation_prob"`
		MutationPert      float64 `json:"mutation_perturbation"`
		TournamentSize    int     `json:"tournament_size"`
	} `json:"operators"`
	CostConfig struct {
		ConcreteUnitPrice    float64 `json:"concrete_unit_price"`
		ClayUnitPrice        float64 `json:"clay_unit_price"`
		GeomembraneUnitPrice float64 `json:"geomembrane_unit_price"`
		ExcavationUnitPrice  float64 `json:"excavation_unit_price"`
		MaxBudget            float64 `json:"max_budget"`
	} `json:"cost_config"`
	Parallel struct {
		Enabled    bool `json:"enabled"`
		MaxWorkers int  `json:"max_workers"`
	} `json:"parallel"`
}

type AntiSeepageOptimizer struct {
	cfg         GeneticConfig
	simCfgPath  string
	store       *database.DataStore
	bus         *message.Bus
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
}

func LoadConfig(path string) (GeneticConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GeneticConfig{}, err
	}
	var c GeneticConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return GeneticConfig{}, err
	}
	return c, nil
}

func New(cfg GeneticConfig, simCfgPath string, store *database.DataStore, bus *message.Bus) *AntiSeepageOptimizer {
	ctx, cancel := context.WithCancel(context.Background())
	return &AntiSeepageOptimizer{
		cfg:        cfg,
		simCfgPath: simCfgPath,
		store:      store,
		bus:        bus,
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (o *AntiSeepageOptimizer) Start() {
	o.wg.Add(1)
	go o.loop()
	log.Println("[anti_seepage_optimizer] started")
}

func (o *AntiSeepageOptimizer) Stop() {
	o.cancel()
	o.wg.Wait()
	log.Println("[anti_seepage_optimizer] stopped")
}

func (o *AntiSeepageOptimizer) loop() {
	defer o.wg.Done()
	for {
		select {
		case <-o.ctx.Done():
			return
		case req, ok := <-o.bus.OptRequestCh:
			if !ok {
				return
			}
			res := o.Run(req)
			if req.ResponseCh != nil {
				select {
				case req.ResponseCh <- res:
				default:
					log.Printf("[anti_seepage_optimizer] response channel full for %s", req.RequestID)
				}
			}
		}
	}
}

func (o *AntiSeepageOptimizer) Run(req message.OptRequestMsg) *message.OptResultMsg {
	o.mu.Lock()
	defer o.mu.Unlock()

	start := time.Now()

	gaParams := optimization.GAParams{
		PopSize:          o.cfg.PopulationSize,
		MaxGen:           o.cfg.Generations,
		MutationRate:     o.cfg.Operators.MutationProb,
		CrossoverRate:    o.cfg.Operators.SBXCrossoverProb,
		TournamentSize:   o.cfg.Operators.TournamentSize,
		MinLen:           o.cfg.DecisionVariables.BlanketLength.Min,
		MaxLen:           o.cfg.DecisionVariables.BlanketLength.Max,
		MinThick:         o.cfg.DecisionVariables.BlanketThickness.Min,
		MaxThick:         o.cfg.DecisionVariables.BlanketThickness.Max,
		Cost: optimization.CostConfig{
			ConcreteUnitPrice:    o.cfg.CostConfig.ConcreteUnitPrice,
			ClayUnitPrice:        o.cfg.CostConfig.ClayUnitPrice,
			GeomembraneUnitPrice: o.cfg.CostConfig.GeomembraneUnitPrice,
			ExcavationUnitPrice:  o.cfg.CostConfig.ExcavationUnitPrice,
			MaxBudget:            o.cfg.CostConfig.MaxBudget,
		},
	}

	opt := optimization.NewGeneticOptimizerFromConfig(o.simCfgPath, gaParams)

	result, pareto, err := opt.OptimizeMulti(
		req.UpstreamWL,
		req.DownstreamWL,
		req.RequestID,
	)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return &message.OptResultMsg{
			RequestID: req.RequestID,
			Success:   false,
			Error:     err.Error(),
			ElapsedMs: elapsed,
		}
	}

	result.OptimizationTimeMs = elapsed

	if o.store != nil {
		ctxDB, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		id, dbErr := o.store.InsertOptimizationResult(ctxDB, result)
		if dbErr == nil {
			result.ID = id
		}
	}

	paretoModels := make([]models.ParetoSolution, len(pareto))
	for i, p := range pareto {
		paretoModels[i] = models.ParetoSolution{
			BlanketLength:    p.BlanketLength,
			BlanketThickness: p.BlanketThickness,
			SeepageFlow:      p.SeepageFlow,
			MaterialCost:     p.MaterialCost,
			FlowReduction:    p.FlowReduction,
			Rank:             p.Rank,
		}
	}

	return &message.OptResultMsg{
		RequestID:   req.RequestID,
		Success:     true,
		Result:      result,
		ParetoFront: paretoModels,
		ElapsedMs:   elapsed,
	}
}
