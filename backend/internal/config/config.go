package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"tashan-weir-seepage/internal/simulation"
)

type AppConfig struct {
	Hydraulics HydraulicsConfig `json:"-"`
	Genetic    GeneticConfig    `json:"-"`
}

type HydraulicsConfig struct {
	DamGeometry  simulation.DamGeometry `json:"dam_geometry"`
	Hydrology    HydrologyConfig        `json:"hydrology"`
	Seepage      SeepageConfig          `json:"seepage"`
	AlarmRules   AlarmRulesConfig       `json:"alarm_rules"`
	MaterialCost MaterialCostConfig     `json:"material_cost"`
}

type HydrologyConfig struct {
	DefaultUpstreamWL   float64 `json:"default_upstream_wl"`
	DefaultDownstreamWL float64 `json:"default_downstream_wl"`
	WaterDensity        float64 `json:"water_density"`
	Gravity             float64 `json:"gravity"`
}

type SeepageConfig struct {
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
}

type AlarmRulesConfig struct {
	PiezometerWarning float64 `json:"piezometer_warning"`
	PiezometerDanger  float64 `json:"piezometer_danger"`
	SeepageWarning    float64 `json:"seepage_warning"`
	SeepageDanger     float64 `json:"seepage_danger"`
	ScourDepthWarning float64 `json:"scour_depth_warning"`
	ScourDepthDanger  float64 `json:"scour_depth_danger"`
	WaterLevelWarning float64 `json:"water_level_warning"`
	WaterLevelDanger  float64 `json:"water_level_danger"`
}

type MaterialCostConfig struct {
	ConcretePerCubicMeter    float64 `json:"concrete_per_cubic_meter"`
	ClayCorePerCubicMeter    float64 `json:"clay_core_per_cubic_meter"`
	GeomembranePerSquareMeter float64 `json:"geomembrane_per_square_meter"`
	ExcavationPerCubicMeter  float64 `json:"excavation_per_cubic_meter"`
	MaxBudget                float64 `json:"max_budget"`
}

type GeneticConfig struct {
	Algorithm         string                 `json:"algorithm"`
	PopulationSize    int                    `json:"population_size"`
	Generations       int                    `json:"generations"`
	DecisionVariables DecisionVariablesConfig `json:"decision_variables"`
	Operators         OperatorsConfig        `json:"operators"`
	CostConfig        GeneticCostConfig      `json:"cost_config"`
	Parallel          ParallelConfig         `json:"parallel"`
}

type DecisionVariablesConfig struct {
	BlanketLength   MinMaxConfig `json:"blanket_length"`
	BlanketThickness MinMaxConfig `json:"blanket_thickness"`
}

type MinMaxConfig struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type OperatorsConfig struct {
	SBXEtaC          float64 `json:"sbx_eta_c"`
	SBXCrossoverProb float64 `json:"sbx_crossover_prob"`
	PolynomialEtaM   float64 `json:"polynomial_eta_m"`
	MutationProb     float64 `json:"mutation_prob"`
	MutationPert     float64 `json:"mutation_perturbation"`
	TournamentSize   int     `json:"tournament_size"`
}

type GeneticCostConfig struct {
	ConcreteUnitPrice    float64 `json:"concrete_unit_price"`
	ClayUnitPrice        float64 `json:"clay_unit_price"`
	GeomembraneUnitPrice float64 `json:"geomembrane_unit_price"`
	ExcavationUnitPrice  float64 `json:"excavation_unit_price"`
	MaxBudget            float64 `json:"max_budget"`
}

type ParallelConfig struct {
	Enabled    bool `json:"enabled"`
	MaxWorkers int  `json:"max_workers"`
}

type NSGAParamsConfig struct {
	PopulationSize int     `json:"population_size"`
	CrossoverRate  float64 `json:"crossover_rate"`
	MutationRate   float64 `json:"mutation_rate"`
	MaxGenerations int     `json:"max_generations"`
}

func Load(hydraulicsPath, geneticPath string) (*AppConfig, error) {
	hydraData, err := os.ReadFile(hydraulicsPath)
	if err != nil {
		return nil, err
	}

	genData, err := os.ReadFile(geneticPath)
	if err != nil {
		return nil, err
	}

	var hydra HydraulicsConfig
	if err := json.Unmarshal(hydraData, &hydra); err != nil {
		return nil, err
	}

	var gen GeneticConfig
	if err := json.Unmarshal(genData, &gen); err != nil {
		return nil, err
	}

	setDefaults(&hydra, &gen)

	return &AppConfig{
		Hydraulics: hydra,
		Genetic:    gen,
	}, nil
}

func setDefaults(hydra *HydraulicsConfig, gen *GeneticConfig) {
	if hydra.AlarmRules.PiezometerWarning == 0 {
		hydra.AlarmRules.PiezometerWarning = 50.0
	}
	if hydra.AlarmRules.PiezometerDanger == 0 {
		hydra.AlarmRules.PiezometerDanger = 70.0
	}
	if hydra.AlarmRules.SeepageWarning == 0 {
		hydra.AlarmRules.SeepageWarning = 5.0
	}
	if hydra.AlarmRules.SeepageDanger == 0 {
		hydra.AlarmRules.SeepageDanger = 10.0
	}
	if hydra.AlarmRules.ScourDepthWarning == 0 {
		hydra.AlarmRules.ScourDepthWarning = 1.5
	}
	if hydra.AlarmRules.ScourDepthDanger == 0 {
		hydra.AlarmRules.ScourDepthDanger = 2.5
	}
	if hydra.AlarmRules.WaterLevelWarning == 0 {
		hydra.AlarmRules.WaterLevelWarning = 7.5
	}
	if hydra.AlarmRules.WaterLevelDanger == 0 {
		hydra.AlarmRules.WaterLevelDanger = 9.0
	}
	if hydra.MaterialCost.ConcretePerCubicMeter == 0 {
		hydra.MaterialCost.ConcretePerCubicMeter = 350.0
	}
	if hydra.MaterialCost.ClayCorePerCubicMeter == 0 {
		hydra.MaterialCost.ClayCorePerCubicMeter = 120.0
	}
	if hydra.MaterialCost.GeomembranePerSquareMeter == 0 {
		hydra.MaterialCost.GeomembranePerSquareMeter = 85.0
	}
	if hydra.MaterialCost.ExcavationPerCubicMeter == 0 {
		hydra.MaterialCost.ExcavationPerCubicMeter = 45.0
	}
	if hydra.MaterialCost.MaxBudget == 0 {
		hydra.MaterialCost.MaxBudget = 500000.0
	}
}

func ResolvePath(p string) string {
	if _, err := os.Stat(p); err == nil {
		return p
	}
	alt := filepath.Join("..", p)
	if _, err := os.Stat(alt); err == nil {
		return alt
	}
	return p
}
