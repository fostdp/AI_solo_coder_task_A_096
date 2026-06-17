package optimization

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	"tashan-weir-seepage/internal/models"
	"tashan-weir-seepage/internal/simulation"
	"tashan-weir-seepage/pkg/utils"
)

type CostConfig struct {
	ConcreteUnitPrice  float64
	ClayUnitPrice      float64
	GeomembraneUnitPrice float64
	ExcavationUnitPrice  float64
	MaxBudget          float64
}

type Individual struct {
	BlanketLength    float64
	BlanketThickness float64
	Fitness          float64
	SeepageFlow      float64
	MaterialCost     float64
	Valid            bool
	Rank             int
	CrowdingDistance  float64
}

type ParetoSolution struct {
	BlanketLength    float64
	BlanketThickness float64
	SeepageFlow      float64
	MaterialCost     float64
	FlowReduction    float64
	Rank             int
}

type GeneticOptimizer struct {
	PopulationSize      int
	MaxGenerations      int
	MutationRate        float64
	CrossoverRate       float64
	TournamentSize      int
	MinBlanketLength    float64
	MaxBlanketLength    float64
	MinBlanketThickness float64
	MaxBlanketThickness float64
	BlanketPermeability float64
	BaseSolver          *simulation.SeepageSolver
	UpstreamH           float64
	DownstreamH         float64
	ConvergenceCurve   []float64
	CostConfig          CostConfig
	ParetoFront         []ParetoSolution
	rand                *rand.Rand
}

func NewGeneticOptimizer(geo simulation.DamGeometry, basePermeability float64) *GeneticOptimizer {
	solver := simulation.NewSeepageSolver(geo, basePermeability)
	solver.SetGridResolution(40, 20)

	return &GeneticOptimizer{
		PopulationSize:      50,
		MaxGenerations:      100,
		MutationRate:        0.1,
		CrossoverRate:       0.8,
		TournamentSize:      3,
		MinBlanketLength:    2.0,
		MaxBlanketLength:    30.0,
		MinBlanketThickness: 0.3,
		MaxBlanketThickness: 3.0,
		BlanketPermeability: basePermeability * 0.01,
		BaseSolver:          solver,
		rand:                utils.NewRand(),
		CostConfig: CostConfig{
			ConcreteUnitPrice:    350.0,
			ClayUnitPrice:        120.0,
			GeomembraneUnitPrice: 85.0,
			ExcavationUnitPrice:  45.0,
			MaxBudget:            500000.0,
		},
	}
}

type HydraulicsConfigLite struct {
	DamGeometry simulation.DamGeometry `json:"dam_geometry"`
	Seepage     struct {
		BasePermeability           float64 `json:"base_permeability"`
		FoundationPermeabilityRatio float64 `json:"foundation_permeability_ratio"`
		InterfaceEnabled           bool    `json:"interface_enabled"`
		InterfaceThicknessRatio    float64 `json:"interface_thickness_ratio"`
		InterfacePermeabilityRatio float64 `json:"interface_permeability_ratio"`
		BlanketPermeabilityRatio   float64 `json:"blanket_permeability_ratio"`
		GridNX                     int     `json:"grid_nx"`
		GridNY                     int     `json:"grid_ny"`
	} `json:"seepage"`
}

type GAParams struct {
	PopSize, MaxGen             int
	MutationRate, CrossoverRate float64
	TournamentSize              int
	MinLen, MaxLen              float64
	MinThick, MaxThick          float64
	BlanketPermRatio            float64
	Cost                        CostConfig
}

func NewGeneticOptimizerFromConfig(hydraulicsCfgPath string, gaCfg interface{}) *GeneticOptimizer {
	var hydCfg HydraulicsConfigLite
	if hydraulicsCfgPath != "" {
		if data, err := os.ReadFile(hydraulicsCfgPath); err == nil {
			_ = json.Unmarshal(data, &hydCfg)
		}
	}
	if hydCfg.Seepage.BasePermeability <= 0 {
		hydCfg.Seepage.BasePermeability = 1e-7
	}
	if hydCfg.Seepage.GridNX <= 0 {
		hydCfg.Seepage.GridNX = 60
	}
	if hydCfg.Seepage.GridNY <= 0 {
		hydCfg.Seepage.GridNY = 30
	}
	if hydCfg.Seepage.FoundationPermeabilityRatio <= 0 {
		hydCfg.Seepage.FoundationPermeabilityRatio = 5.0
	}

	solver := simulation.NewSeepageSolverWithConfig(
		hydCfg.DamGeometry,
		hydCfg.Seepage.BasePermeability,
		hydCfg.Seepage.GridNX,
		hydCfg.Seepage.GridNY,
		hydCfg.Seepage.FoundationPermeabilityRatio,
		hydCfg.Seepage.InterfaceEnabled,
		hydCfg.Seepage.InterfaceThicknessRatio,
		hydCfg.Seepage.InterfacePermeabilityRatio,
	)

	blanketRatio := hydCfg.Seepage.BlanketPermeabilityRatio
	if blanketRatio <= 0 {
		blanketRatio = 0.01
	}

	defaultGA := GAParams{
		PopSize:           60,
		MaxGen:            80,
		MutationRate:      0.1,
		CrossoverRate:     0.9,
		TournamentSize:    2,
		MinLen:            1.0,
		MaxLen:            20.0,
		MinThick:          0.2,
		MaxThick:          3.0,
		BlanketPermRatio:  blanketRatio,
		Cost: CostConfig{
			ConcreteUnitPrice:    350,
			ClayUnitPrice:        120,
			GeomembraneUnitPrice: 85,
			ExcavationUnitPrice:  45,
			MaxBudget:            500000,
		},
	}

	if cfg, ok := gaCfg.(GAParams); ok {
		if cfg.PopSize > 0 {
			defaultGA.PopSize = cfg.PopSize
		}
		if cfg.MaxGen > 0 {
			defaultGA.MaxGen = cfg.MaxGen
		}
		if cfg.MutationRate > 0 {
			defaultGA.MutationRate = cfg.MutationRate
		}
		if cfg.CrossoverRate > 0 {
			defaultGA.CrossoverRate = cfg.CrossoverRate
		}
		if cfg.TournamentSize > 0 {
			defaultGA.TournamentSize = cfg.TournamentSize
		}
		if cfg.MinLen > 0 {
			defaultGA.MinLen = cfg.MinLen
		}
		if cfg.MaxLen > cfg.MinLen {
			defaultGA.MaxLen = cfg.MaxLen
		}
		if cfg.MinThick > 0 {
			defaultGA.MinThick = cfg.MinThick
		}
		if cfg.MaxThick > cfg.MinThick {
			defaultGA.MaxThick = cfg.MaxThick
		}
		if cfg.BlanketPermRatio > 0 {
			defaultGA.BlanketPermRatio = cfg.BlanketPermRatio
		}
		if cfg.Cost.ConcreteUnitPrice > 0 {
			defaultGA.Cost = cfg.Cost
		}
	}

	return &GeneticOptimizer{
		PopulationSize:      defaultGA.PopSize,
		MaxGenerations:      defaultGA.MaxGen,
		MutationRate:        defaultGA.MutationRate,
		CrossoverRate:       defaultGA.CrossoverRate,
		TournamentSize:      defaultGA.TournamentSize,
		MinBlanketLength:    defaultGA.MinLen,
		MaxBlanketLength:    defaultGA.MaxLen,
		MinBlanketThickness: defaultGA.MinThick,
		MaxBlanketThickness: defaultGA.MaxThick,
		BlanketPermeability: hydCfg.Seepage.BasePermeability * defaultGA.BlanketPermRatio,
		BaseSolver:          solver,
		rand:                utils.NewRand(),
		CostConfig:          defaultGA.Cost,
	}
}

func (ga *GeneticOptimizer) Configure(req models.OptimizationRequest) {
	if req.MinBlanketLength > 0 {
		ga.MinBlanketLength = req.MinBlanketLength
	}
	if req.MaxBlanketLength > 0 {
		ga.MaxBlanketLength = req.MaxBlanketLength
	}
	if req.MinBlanketThickness > 0 {
		ga.MinBlanketThickness = req.MinBlanketThickness
	}
	if req.MaxBlanketThickness > 0 {
		ga.MaxBlanketThickness = req.MaxBlanketThickness
	}
	if req.PopulationSize > 0 {
		ga.PopulationSize = req.PopulationSize
	}
	if req.MaxGenerations > 0 {
		ga.MaxGenerations = req.MaxGenerations
	}
	if req.MutationRate > 0 {
		ga.MutationRate = req.MutationRate
	}
	if req.CrossoverRate > 0 {
		ga.CrossoverRate = req.CrossoverRate
	}

	ga.UpstreamH = req.UpstreamWaterLevel
	ga.DownstreamH = req.DownstreamWaterLevel
}

func (ga *GeneticOptimizer) evaluateIndividual(ind *Individual) {
	simReq := models.SimulationRequest{
		UpstreamWaterLevel:   ga.UpstreamH,
		DownstreamWaterLevel: ga.DownstreamH,
		GridResolutionX:      40,
		GridResolutionY:      20,
		PermeabilityK:        ga.BaseSolver.PermeabilityK,
		BlanketLength:        &ind.BlanketLength,
		BlanketThickness:     &ind.BlanketThickness,
	}

	localSolver := *ga.BaseSolver
	simResult, _, err := localSolver.RunSimulation(simReq)
	if err != nil {
		ind.Valid = false
		ind.Fitness = 1e20
		ind.Rank = 999
		return
	}

	ind.SeepageFlow = simResult.TotalSeepageFlow
	ind.MaterialCost = ga.calculateMaterialCost(ind.BlanketLength, ind.BlanketThickness)
	ind.Valid = true

	if ga.CostConfig.MaxBudget > 0 && ind.MaterialCost > ga.CostConfig.MaxBudget {
		ind.Valid = false
		ind.Fitness = 1e20
		ind.Rank = 998
		return
	}

	ind.Rank = 0
	ind.CrowdingDistance = 0
	ind.Fitness = 0
}

func (ga *GeneticOptimizer) calculateMaterialCost(length, thickness float64) float64 {
	blanketArea := length * thickness * 15.0
	clayVolume := length * thickness * 15.0
	excavationVolume := length * 1.0 * 15.0

	geomembraneArea := length * 15.0

	cost := 0.0
	cost += blanketArea * ga.CostConfig.ConcreteUnitPrice
	cost += clayVolume * ga.CostConfig.ClayUnitPrice
	cost += geomembraneArea * ga.CostConfig.GeomembraneUnitPrice
	cost += excavationVolume * ga.CostConfig.ExcavationUnitPrice

	return cost
}

func (ga *GeneticOptimizer) dominates(a, b Individual) bool {
	if !a.Valid && b.Valid {
		return false
	}
	if a.Valid && !b.Valid {
		return true
	}
	if !a.Valid && !b.Valid {
		return false
	}

	aBetterFlow := a.SeepageFlow < b.SeepageFlow
	aBetterCost := a.MaterialCost < b.MaterialCost
	aEqualFlow := math.Abs(a.SeepageFlow-b.SeepageFlow) < 1e-15
	aEqualCost := math.Abs(a.MaterialCost-b.MaterialCost) < 1e-6

	if (aBetterFlow || aEqualFlow) && (aBetterCost || aEqualCost) && !(aEqualFlow && aEqualCost) {
		return true
	}
	return false
}

func (ga *GeneticOptimizer) fastNonDominatedSort(population []Individual) [][]int {
	n := len(population)
	dominationCount := make([]int, n)
	dominatedSet := make([][]int, n)
	ranks := make([][]int, 0)

	for i := 0; i < n; i++ {
		dominatedSet[i] = make([]int, 0)
		dominationCount[i] = 0
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if ga.dominates(population[i], population[j]) {
				dominatedSet[i] = append(dominatedSet[i], j)
				dominationCount[j]++
			} else if ga.dominates(population[j], population[i]) {
				dominatedSet[j] = append(dominatedSet[j], i)
				dominationCount[i]++
			}
		}
	}

	currentFront := make([]int, 0)
	for i := 0; i < n; i++ {
		if dominationCount[i] == 0 {
			population[i].Rank = 0
			currentFront = append(currentFront, i)
		}
	}

	rank := 0
	for len(currentFront) > 0 {
		ranks = append(ranks, make([]int, len(currentFront)))
		copy(ranks[rank], currentFront)

		nextFront := make([]int, 0)
		for _, i := range currentFront {
			for _, j := range dominatedSet[i] {
				dominationCount[j]--
				if dominationCount[j] == 0 {
					population[j].Rank = rank + 1
					nextFront = append(nextFront, j)
				}
			}
		}

		rank++
		currentFront = nextFront
	}

	return ranks
}

func (ga *GeneticOptimizer) crowdingDistanceAssignment(population []Individual, front []int) {
	if len(front) <= 2 {
		for _, idx := range front {
			population[idx].CrowdingDistance = 1e18
		}
		return
	}

	for _, idx := range front {
		population[idx].CrowdingDistance = 0
	}

	objectives := []func(Individual) float64{
		func(ind Individual) float64 { return ind.SeepageFlow },
		func(ind Individual) float64 { return ind.MaterialCost },
	}

	for _, objFn := range objectives {
		sorted := make([]int, len(front))
		copy(sorted, front)
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if objFn(population[sorted[i]]) > objFn(population[sorted[j]]) {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}

		objMin := objFn(population[sorted[0]])
		objMax := objFn(population[sorted[len(sorted)-1]])
		objRange := objMax - objMin

		population[sorted[0]].CrowdingDistance = 1e18
		population[sorted[len(sorted)-1]].CrowdingDistance = 1e18

		if objRange > 0 {
			for i := 1; i < len(sorted)-1; i++ {
				prev := objFn(population[sorted[i-1]])
				next := objFn(population[sorted[i+1]])
				population[sorted[i]].CrowdingDistance += (next - prev) / objRange
			}
		}
	}
}

func (ga *GeneticOptimizer) nsga2TournamentSelect(population []Individual) Individual {
	bestIdx := ga.rand.Intn(len(population))
	for i := 1; i < ga.TournamentSize; i++ {
		idx := ga.rand.Intn(len(population))
		if ga.nsga2Compare(population[idx], population[bestIdx]) < 0 {
			bestIdx = idx
		}
	}
	return population[bestIdx]
}

func (ga *GeneticOptimizer) nsga2Compare(a, b Individual) int {
	if !a.Valid && b.Valid {
		return 1
	}
	if a.Valid && !b.Valid {
		return -1
	}
	if !a.Valid && !b.Valid {
		return 0
	}

	if a.Rank < b.Rank {
		return -1
	}
	if a.Rank > b.Rank {
		return 1
	}

	if a.CrowdingDistance > b.CrowdingDistance {
		return -1
	}
	if a.CrowdingDistance < b.CrowdingDistance {
		return 1
	}
	return 0
}

func (ga *GeneticOptimizer) extractParetoFront(population []Individual, baselineFlow float64) []ParetoSolution {
	fronts := ga.fastNonDominatedSort(population)
	if len(fronts) == 0 {
		return nil
	}

	var pareto []ParetoSolution
	for _, idx := range fronts[0] {
		ind := population[idx]
		if !ind.Valid {
			continue
		}
		reduction := 0.0
		if baselineFlow > 0 {
			reduction = (baselineFlow - ind.SeepageFlow) / baselineFlow * 100
		}
		pareto = append(pareto, ParetoSolution{
			BlanketLength:    ind.BlanketLength,
			BlanketThickness: ind.BlanketThickness,
			SeepageFlow:      ind.SeepageFlow,
			MaterialCost:     ind.MaterialCost,
			FlowReduction:    reduction,
		})
	}
	return pareto
}

func (ga *GeneticOptimizer) createInitialPopulation() []Individual {
	population := make([]Individual, ga.PopulationSize)
	for i := 0; i < ga.PopulationSize; i++ {
		length := ga.MinBlanketLength + ga.rand.Float64()*(ga.MaxBlanketLength-ga.MinBlanketLength)
		thickness := ga.MinBlanketThickness + ga.rand.Float64()*(ga.MaxBlanketThickness-ga.MinBlanketThickness)
		population[i] = Individual{
			BlanketLength:    length,
			BlanketThickness: thickness,
		}
	}
	return population
}

func (ga *GeneticOptimizer) tournamentSelect(population []Individual) Individual {
	bestIdx := ga.rand.Intn(len(population))
	for i := 1; i < ga.TournamentSize; i++ {
		idx := ga.rand.Intn(len(population))
		if population[idx].Fitness < population[bestIdx].Fitness {
			bestIdx = idx
		}
	}
	return population[bestIdx]
}

func (ga *GeneticOptimizer) crossover(parent1, parent2 Individual) (Individual, Individual) {
	child1 := parent1
	child2 := parent2

	if ga.rand.Float64() < ga.CrossoverRate {
		alpha := ga.rand.Float64()
		child1.BlanketLength = alpha*parent1.BlanketLength + (1-alpha)*parent2.BlanketLength
		child2.BlanketLength = (1-alpha)*parent1.BlanketLength + alpha*parent2.BlanketLength

		beta := ga.rand.Float64()
		child1.BlanketThickness = beta*parent1.BlanketThickness + (1-beta)*parent2.BlanketThickness
		child2.BlanketThickness = (1-beta)*parent1.BlanketThickness + beta*parent2.BlanketThickness
	}

	child1.BlanketLength = utils.Clamp(child1.BlanketLength, ga.MinBlanketLength, ga.MaxBlanketLength)
	child1.BlanketThickness = utils.Clamp(child1.BlanketThickness, ga.MinBlanketThickness, ga.MaxBlanketThickness)
	child2.BlanketLength = utils.Clamp(child2.BlanketLength, ga.MinBlanketLength, ga.MaxBlanketLength)
	child2.BlanketThickness = utils.Clamp(child2.BlanketThickness, ga.MinBlanketThickness, ga.MaxBlanketThickness)

	return child1, child2
}

func (ga *GeneticOptimizer) mutate(ind Individual) Individual {
	if ga.rand.Float64() < ga.MutationRate {
		mutationAmount := (ga.MaxBlanketLength - ga.MinBlanketLength) * 0.2
		ind.BlanketLength += ga.rand.NormFloat64() * mutationAmount
		ind.BlanketLength = utils.Clamp(ind.BlanketLength, ga.MinBlanketLength, ga.MaxBlanketLength)
	}

	if ga.rand.Float64() < ga.MutationRate {
		mutationAmount := (ga.MaxBlanketThickness - ga.MinBlanketThickness) * 0.2
		ind.BlanketThickness += ga.rand.NormFloat64() * mutationAmount
		ind.BlanketThickness = utils.Clamp(ind.BlanketThickness, ga.MinBlanketThickness, ga.MaxBlanketThickness)
	}

	return ind
}

func (ga *GeneticOptimizer) getBestIndividual(population []Individual) Individual {
	best := population[0]
	for _, ind := range population[1:] {
		if ind.Fitness < best.Fitness {
			best = ind
		}
	}
	return best
}

func (ga *GeneticOptimizer) getAverageFitness(population []Individual) float64 {
	sum := 0.0
	count := 0
	for _, ind := range population {
		if ind.Valid {
			sum += ind.Fitness
			count++
		}
	}
	if count == 0 {
		return 1e20
	}
	return sum / float64(count)
}

func (ga *GeneticOptimizer) calculateBaselineFlow() float64 {
	simReq := models.SimulationRequest{
		UpstreamWaterLevel:   ga.UpstreamH,
		DownstreamWaterLevel: ga.DownstreamH,
		GridResolutionX:      60,
		GridResolutionY:      30,
		PermeabilityK:        ga.BaseSolver.PermeabilityK,
	}

	simResult, _, err := ga.BaseSolver.RunSimulation(simReq)
	if err != nil {
		return 0
	}
	return simResult.TotalSeepageFlow
}

func (ga *GeneticOptimizer) Optimize(req models.OptimizationRequest) (*models.OptimizationResult, error) {
	startTime := time.Now()

	ga.Configure(req)
	baselineFlow := ga.calculateBaselineFlow()

	population := ga.createInitialPopulation()

	ga.ConvergenceCurve = make([]float64, 0, ga.MaxGenerations)

	for i := range population {
		ga.evaluateIndividual(&population[i])
	}

	fronts := ga.fastNonDominatedSort(population)
	for _, front := range fronts {
		ga.crowdingDistanceAssignment(population, front)
	}

	ga.ParetoFront = ga.extractParetoFront(population, baselineFlow)

	for gen := 0; gen < ga.MaxGenerations; gen++ {
		offspring := make([]Individual, 0, ga.PopulationSize)

		for len(offspring) < ga.PopulationSize {
			parent1 := ga.nsga2TournamentSelect(population)
			parent2 := ga.nsga2TournamentSelect(population)

			child1, child2 := ga.crossover(parent1, parent2)
			child1 = ga.mutate(child1)
			child2 = ga.mutate(child2)

			ga.evaluateIndividual(&child1)
			ga.evaluateIndividual(&child2)

			offspring = append(offspring, child1)
			if len(offspring) < ga.PopulationSize {
				offspring = append(offspring, child2)
			}
		}

		combined := make([]Individual, 0, len(population)+len(offspring))
		combined = append(combined, population...)
		combined = append(combined, offspring...)

		fronts = ga.fastNonDominatedSort(combined)
		for _, front := range fronts {
			ga.crowdingDistanceAssignment(combined, front)
		}

		newPopulation := make([]Individual, 0, ga.PopulationSize)
		for _, front := range fronts {
			if len(newPopulation)+len(front) <= ga.PopulationSize {
				newPopulation = append(newPopulation, combined[front[0]:front[len(front)-1]+1]...)
				if len(newPopulation) >= ga.PopulationSize {
					break
				}
			} else {
				sortedFront := make([]int, len(front))
				copy(sortedFront, front)
				for i := 0; i < len(sortedFront)-1; i++ {
					for j := i + 1; j < len(sortedFront); j++ {
						if ga.nsga2Compare(combined[sortedFront[j]], combined[sortedFront[i]]) < 0 {
							sortedFront[i], sortedFront[j] = sortedFront[j], sortedFront[i]
						}
					}
				}
				for _, idx := range sortedFront {
					newPopulation = append(newPopulation, combined[idx])
					if len(newPopulation) >= ga.PopulationSize {
						break
					}
				}
				break
			}
		}

		if len(newPopulation) > ga.PopulationSize {
			newPopulation = newPopulation[:ga.PopulationSize]
		}
		population = newPopulation

		ga.ParetoFront = ga.extractParetoFront(population, baselineFlow)

		var bestFlow float64 = 1e20
		for _, p := range ga.ParetoFront {
			if p.SeepageFlow < bestFlow {
				bestFlow = p.SeepageFlow
			}
		}
		ga.ConvergenceCurve = append(ga.ConvergenceCurve, bestFlow)

		if gen >= 20 {
			improved := false
			recentBest := ga.ConvergenceCurve[len(ga.ConvergenceCurve)-1]
			for i := len(ga.ConvergenceCurve) - 20; i < len(ga.ConvergenceCurve); i++ {
				if math.Abs(recentBest-ga.ConvergenceCurve[i]) > math.Abs(recentBest)*0.001 {
					improved = true
					break
				}
			}
			if !improved {
				break
			}
		}
	}

	ga.ParetoFront = ga.extractParetoFront(population, baselineFlow)

	var bestOverall Individual
	bestOverall.SeepageFlow = 1e20
	for _, ind := range population {
		if ind.Valid && ind.SeepageFlow < bestOverall.SeepageFlow {
			bestOverall = ind
		}
	}

	if !bestOverall.Valid {
		for _, ind := range population {
			if ind.Valid {
				bestOverall = ind
				break
			}
		}
	}

	finalSolver := *ga.BaseSolver
	finalSolver.SetGridResolution(60, 30)
	finalSolver.SetBlanket(bestOverall.BlanketLength, bestOverall.BlanketThickness, ga.BlanketPermeability)

	finalSimReq := models.SimulationRequest{
		UpstreamWaterLevel:   ga.UpstreamH,
		DownstreamWaterLevel: ga.DownstreamH,
		GridResolutionX:      60,
		GridResolutionY:      30,
		PermeabilityK:        ga.BaseSolver.PermeabilityK,
		BlanketLength:        &bestOverall.BlanketLength,
		BlanketThickness:     &bestOverall.BlanketThickness,
	}
	finalSimResult, _, _ := finalSolver.RunSimulation(finalSimReq)

	optTime := time.Since(startTime).Milliseconds()

	var reductionRate float64
	if baselineFlow > 0 {
		reductionRate = (baselineFlow - finalSimResult.TotalSeepageFlow) / baselineFlow * 100
	}

	paretoData := make([]map[string]interface{}, 0)
	for _, p := range ga.ParetoFront {
		paretoData = append(paretoData, map[string]interface{}{
			"blanket_length":    p.BlanketLength,
			"blanket_thickness": p.BlanketThickness,
			"seepage_flow":      p.SeepageFlow,
			"material_cost":     p.MaterialCost,
			"flow_reduction":    p.FlowReduction,
		})
	}

	result := &models.OptimizationResult{
		OptimizationName:     req.OptimizationName,
		Algorithm:            "NSGA-II",
		UpstreamWaterLevel:   ga.UpstreamH,
		DownstreamWaterLevel: ga.DownstreamH,
		BlanketLength:        bestOverall.BlanketLength,
		BlanketThickness:     bestOverall.BlanketThickness,
		BlanketPermeability:  ga.BlanketPermeability,
		OptimizedSeepageFlow: finalSimResult.TotalSeepageFlow,
		BaselineSeepageFlow:  baselineFlow,
		FlowReductionRate:    reductionRate,
		GenerationCount:      len(ga.ConvergenceCurve),
		PopulationSize:       ga.PopulationSize,
		BestFitness:          bestOverall.SeepageFlow,
		OptimizationTimeMs:   optTime,
		Parameters: map[string]interface{}{
			"mutation_rate":          ga.MutationRate,
			"crossover_rate":         ga.CrossoverRate,
			"tournament_size":        ga.TournamentSize,
			"blanket_permeability":   ga.BlanketPermeability,
			"base_permeability":      ga.BaseSolver.PermeabilityK,
			"material_cost":          bestOverall.MaterialCost,
			"max_budget":             ga.CostConfig.MaxBudget,
			"budget_constraint":      ga.CostConfig.MaxBudget > 0,
			"pareto_front_size":      len(ga.ParetoFront),
			"pareto_front":           paretoData,
			"cost_config": map[string]interface{}{
				"concrete_unit_price":     ga.CostConfig.ConcreteUnitPrice,
				"clay_unit_price":         ga.CostConfig.ClayUnitPrice,
				"geomembrane_unit_price":  ga.CostConfig.GeomembraneUnitPrice,
				"excavation_unit_price":   ga.CostConfig.ExcavationUnitPrice,
				"max_budget":              ga.CostConfig.MaxBudget,
			},
		},
		ConvergenceCurve: ga.ConvergenceCurve,
	}

	return result, nil
}

func (ga *GeneticOptimizer) OptimizeMulti(upstreamWL, downstreamWL float64, name string) (*models.OptimizationResult, []ParetoSolution, error) {
	req := models.OptimizationRequest{
		UpstreamWaterLevel:   upstreamWL,
		DownstreamWaterLevel: downstreamWL,
		PopulationSize:       ga.PopulationSize,
		MaxGenerations:       ga.MaxGenerations,
		MutationRate:         ga.MutationRate,
		CrossoverRate:        ga.CrossoverRate,
		OptimizationName:     name,
		MinBlanketLength:     ga.MinBlanketLength,
		MaxBlanketLength:     ga.MaxBlanketLength,
		MinBlanketThickness:  ga.MinBlanketThickness,
		MaxBlanketThickness:  ga.MaxBlanketThickness,
	}
	result, err := ga.Optimize(req)
	if err != nil {
		return nil, nil, err
	}
	front := make([]ParetoSolution, len(ga.ParetoFront))
	copy(front, ga.ParetoFront)
	return result, front, nil
}

func (ga *GeneticOptimizer) ValidateOptimization(opt *models.OptimizationResult) error {
	if opt.BlanketLength < ga.MinBlanketLength || opt.BlanketLength > ga.MaxBlanketLength {
		return fmt.Errorf("blanket length %.2f out of range [%.2f, %.2f]",
			opt.BlanketLength, ga.MinBlanketLength, ga.MaxBlanketLength)
	}
	if opt.BlanketThickness < ga.MinBlanketThickness || opt.BlanketThickness > ga.MaxBlanketThickness {
		return fmt.Errorf("blanket thickness %.2f out of range [%.2f, %.2f]",
			opt.BlanketThickness, ga.MinBlanketThickness, ga.MaxBlanketThickness)
	}
	return nil
}
