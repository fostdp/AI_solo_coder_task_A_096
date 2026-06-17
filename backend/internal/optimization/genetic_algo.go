package optimization

import (
	"fmt"
	"math"
	"time"

	"tashan-weir-seepage/internal/models"
	"tashan-weir-seepage/internal/simulation"
	"tashan-weir-seepage/pkg/utils"
)

type Individual struct {
	BlanketLength    float64
	BlanketThickness float64
	Fitness          float64
	SeepageFlow      float64
	Valid            bool
}

type GeneticOptimizer struct {
	PopulationSize     int
	MaxGenerations     int
	MutationRate       float64
	CrossoverRate      float64
	TournamentSize     int
	MinBlanketLength   float64
	MaxBlanketLength   float64
	MinBlanketThickness float64
	MaxBlanketThickness float64
	BlanketPermeability float64
	BaseSolver         *simulation.SeepageSolver
	UpstreamH          float64
	DownstreamH        float64
	ConvergenceCurve   []float64
	rand               *utils.Rand
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
		return
	}

	ind.SeepageFlow = simResult.TotalSeepageFlow
	ind.Valid = true

	volume := ind.BlanketLength * ind.BlanketThickness
	maxVolume := ga.MaxBlanketLength * ga.MaxBlanketThickness

	flowWeight := 0.7
	costWeight := 0.3

	normalizedFlow := ind.SeepageFlow * 1e6
	normalizedCost := (volume / maxVolume) * 100

	ind.Fitness = flowWeight*normalizedFlow + costWeight*normalizedCost
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

	bestOverall := ga.getBestIndividual(population)

	for gen := 0; gen < ga.MaxGenerations; gen++ {
		newPopulation := make([]Individual, 0, ga.PopulationSize)

		elitismCount := 2
		for i := 0; i < elitismCount; i++ {
			newPopulation = append(newPopulation, bestOverall)
		}

		for len(newPopulation) < ga.PopulationSize {
			parent1 := ga.tournamentSelect(population)
			parent2 := ga.tournamentSelect(population)

			child1, child2 := ga.crossover(parent1, parent2)
			child1 = ga.mutate(child1)
			child2 = ga.mutate(child2)

			ga.evaluateIndividual(&child1)
			ga.evaluateIndividual(&child2)

			newPopulation = append(newPopulation, child1)
			if len(newPopulation) < ga.PopulationSize {
				newPopulation = append(newPopulation, child2)
			}
		}

		population = newPopulation[:ga.PopulationSize]

		bestInGen := ga.getBestIndividual(population)
		if bestInGen.Fitness < bestOverall.Fitness {
			bestOverall = bestInGen
		}

		ga.ConvergenceCurve = append(ga.ConvergenceCurve, bestOverall.Fitness)

		if gen >= 20 {
			improved := false
			recentBest := ga.ConvergenceCurve[len(ga.ConvergenceCurve)-1]
			for i := len(ga.ConvergenceCurve) - 20; i < len(ga.ConvergenceCurve); i++ {
				if math.Abs(recentBest-ga.ConvergenceCurve[i]) > bestOverall.Fitness*0.001 {
					improved = true
					break
				}
			}
			if !improved {
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

	result := &models.OptimizationResult{
		OptimizationName:     req.OptimizationName,
		Algorithm:            "genetic_algorithm",
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
		BestFitness:          bestOverall.Fitness,
		OptimizationTimeMs:   optTime,
		Parameters: map[string]interface{}{
			"mutation_rate":          ga.MutationRate,
			"crossover_rate":         ga.CrossoverRate,
			"tournament_size":        ga.TournamentSize,
			"blanket_permeability":   ga.BlanketPermeability,
			"base_permeability":      ga.BaseSolver.PermeabilityK,
			"average_final_fitness":  ga.getAverageFitness(population),
		},
		ConvergenceCurve: ga.ConvergenceCurve,
	}

	return result, nil
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
