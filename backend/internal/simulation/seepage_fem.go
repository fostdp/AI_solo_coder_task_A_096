package simulation

import (
	"fmt"
	"math"
	"time"

	"tashan-weir-seepage/internal/models"
	"tashan-weir-seepage/pkg/utils"
)

type DamGeometry struct {
	Length              float64
	Height              float64
	TopWidth            float64
	UpstreamSlope       float64
	DownstreamSlope     float64
	FoundationDepth     float64
}

type BlanketConfig struct {
	Enabled        bool
	Length         float64
	Thickness      float64
	Permeability   float64
}

type SeepageSolver struct {
	Geometry        DamGeometry
	PermeabilityK   float64
	UpstreamH       float64
	DownstreamH     float64
	GridNX          int
	GridNY          int
	Blanket         BlanketConfig
	XCoords         []float64
	YCoords         []float64
	WaterHead       [][]float64
	VelocityX       [][]float64
	VelocityY       [][]float64
	PorePressure    [][]float64
	IsDamBody       [][]bool
	IsInDomain      [][]bool
}

func NewSeepageSolver(geo DamGeometry, k float64) *SeepageSolver {
	return &SeepageSolver{
		Geometry:      geo,
		PermeabilityK: k,
		GridNX:        60,
		GridNY:        30,
	}
}

func (s *SeepageSolver) SetGridResolution(nx, ny int) {
	s.GridNX = nx
	s.GridNY = ny
}

func (s *SeepageSolver) SetWaterLevels(upH, downH float64) {
	s.UpstreamH = upH
	s.DownstreamH = downH
}

func (s *SeepageSolver) SetBlanket(length, thickness, permeability float64) {
	s.Blanket = BlanketConfig{
		Enabled:      true,
		Length:       length,
		Thickness:    thickness,
		Permeability: permeability,
	}
}

func (s *SeepageSolver) damProfile(yTop float64) (xStart, xEnd float64) {
	upSlope := s.Geometry.UpstreamSlope
	downSlope := s.Geometry.DownstreamSlope
	topWidth := s.Geometry.TopWidth
	totalLength := s.Geometry.Length

	topCenterX := totalLength / 2.0
	xTopStart := topCenterX - topWidth/2.0
	xTopEnd := topCenterX + topWidth/2.0

	relativeY := s.Geometry.Height - yTop
	if relativeY <= 0 {
		return xTopStart, xTopEnd
	}

	xStart = xTopStart - relativeY*upSlope
	xEnd = xTopEnd + relativeY*downSlope

	if xStart < 0 {
		xStart = 0
	}
	if xEnd > totalLength {
		xEnd = totalLength
	}

	return
}

func (s *SeepageSolver) InitializeGrid() {
	totalLength := s.Geometry.Length
	totalHeight := s.Geometry.Height + s.Geometry.FoundationDepth

	s.XCoords = utils.Linspace(0, totalLength, s.GridNX)
	s.YCoords = utils.Linspace(-s.Geometry.FoundationDepth, s.Geometry.Height, s.GridNY)

	s.WaterHead = make([][]float64, s.GridNY)
	s.VelocityX = make([][]float64, s.GridNY)
	s.VelocityY = make([][]float64, s.GridNY)
	s.PorePressure = make([][]float64, s.GridNY)
	s.IsDamBody = make([][]bool, s.GridNY)
	s.IsInDomain = make([][]bool, s.GridNY)

	for j := 0; j < s.GridNY; j++ {
		s.WaterHead[j] = make([]float64, s.GridNX)
		s.VelocityX[j] = make([]float64, s.GridNX)
		s.VelocityY[j] = make([]float64, s.GridNX)
		s.PorePressure[j] = make([]float64, s.GridNX)
		s.IsDamBody[j] = make([]bool, s.GridNX)
		s.IsInDomain[j] = make([]bool, s.GridNX)
	}

	for j := 0; j < s.GridNY; j++ {
		y := s.YCoords[j]
		xStart, xEnd := s.damProfile(y)
		for i := 0; i < s.GridNX; i++ {
			x := s.XCoords[i]
			s.IsDamBody[j][i] = (x >= xStart && x <= xEnd && y > 0) || (y <= 0 && x >= 0 && x <= totalLength)
			s.IsInDomain[j][i] = s.IsDamBody[j][i]

			if s.Blanket.Enabled && y > 0 && y <= s.Blanket.Thickness && x <= s.Blanket.Length {
				s.IsInDomain[j][i] = true
				s.IsDamBody[j][i] = true
			}
		}
	}
}

func (s *SeepageSolver) SetBoundaryConditions() {
	for j := 0; j < s.GridNY; j++ {
		for i := 0; i < s.GridNX; i++ {
			if !s.IsInDomain[j][i] {
				continue
			}
			x := s.XCoords[i]
			y := s.YCoords[j]

			if x <= s.XCoords[1] && y <= s.UpstreamH && s.IsInDomain[j][i] {
				s.WaterHead[j][i] = s.UpstreamH
			} else if x >= s.XCoords[s.GridNX-2] && y <= s.DownstreamH && s.IsInDomain[j][i] {
				s.WaterHead[j][i] = s.DownstreamH
			} else if y <= s.YCoords[1] {
				averageH := (s.UpstreamH + s.DownstreamH) / 2.0
				frac := x / s.Geometry.Length
				s.WaterHead[j][i] = s.UpstreamH - (s.UpstreamH-s.DownstreamH)*frac*0.3 + averageH*0.1
			} else {
				frac := x / s.Geometry.Length
				s.WaterHead[j][i] = s.UpstreamH - (s.UpstreamH-s.DownstreamH)*frac
			}
		}
	}
}

func (s *SeepageSolver) SolveSteady(maxIter int, tol float64) (int, error) {
	dx := (s.Geometry.Length) / float64(s.GridNX-1)
	dy := (s.Geometry.Height + s.Geometry.FoundationDepth) / float64(s.GridNY-1)

	var iter int
	for iter = 0; iter < maxIter; iter++ {
		maxDelta := 0.0

		for j := 1; j < s.GridNY-1; j++ {
			for i := 1; i < s.GridNX-1; i++ {
				if !s.IsInDomain[j][i] {
					continue
				}

				x := s.XCoords[i]
				y := s.YCoords[j]

				isFixedBC := false
				if x <= s.XCoords[1] && y <= s.UpstreamH {
					s.WaterHead[j][i] = s.UpstreamH
					isFixedBC = true
				} else if x >= s.XCoords[s.GridNX-2] && y <= s.DownstreamH {
					s.WaterHead[j][i] = s.DownstreamH
					isFixedBC = true
				}

				if isFixedBC {
					continue
				}

				isOnBoundary := false
				if y > s.YCoords[s.GridNY-2] {
					isOnBoundary = true
				}

				neighbors := 0
				sumH := 0.0
				permSum := 0.0

				if s.IsInDomain[j][i-1] {
					perm := s.getPermeability(i, j, i-1, j)
					sumH += perm * s.WaterHead[j][i-1]
					permSum += perm
					neighbors++
				}
				if s.IsInDomain[j][i+1] {
					perm := s.getPermeability(i, j, i+1, j)
					sumH += perm * s.WaterHead[j][i+1]
					permSum += perm
					neighbors++
				}
				if s.IsInDomain[j-1][i] {
					perm := s.getPermeability(i, j, i, j-1)
					sumH += perm * s.WaterHead[j-1][i]
					permSum += perm
					neighbors++
				}
				if s.IsInDomain[j+1][i] {
					perm := s.getPermeability(i, j, i, j+1)
					sumH += perm * s.WaterHead[j+1][i]
					permSum += perm
					neighbors++
				}

				if neighbors < 2 {
					continue
				}

				newH := sumH / permSum

				if isOnBoundary {
					if newH < y {
						newH = y
					}
				}

				delta := math.Abs(newH - s.WaterHead[j][i])
				if delta > maxDelta {
					maxDelta = delta
				}
				s.WaterHead[j][i] = newH
			}
		}

		for j := 1; j < s.GridNY-1; j++ {
			for i := 1; i < s.GridNX-1; i++ {
				if s.IsInDomain[j][i] && s.WaterHead[j][i] < s.YCoords[j] {
					_, xEnd := s.damProfile(s.YCoords[j])
					if s.XCoords[i] < xEnd {
						waterSurround := 0.0
						nCount := 0
						if i > 0 && s.WaterHead[j][i-1] > s.YCoords[j] {
							waterSurround += s.WaterHead[j][i-1]
							nCount++
						}
						if i < s.GridNX-1 && s.WaterHead[j][i+1] > s.YCoords[j] {
							waterSurround += s.WaterHead[j][i+1]
							nCount++
						}
						if j > 0 && s.WaterHead[j-1][i] > s.YCoords[j] {
							waterSurround += s.WaterHead[j-1][i]
							nCount++
						}
						if j < s.GridNY-1 && s.WaterHead[j+1][i] > s.YCoords[j] {
							waterSurround += s.WaterHead[j+1][i]
							nCount++
						}
						if nCount > 0 {
							avg := waterSurround / float64(nCount)
							s.WaterHead[j][i] = 0.7*avg + 0.3*s.YCoords[j]
						}
					}
				}
			}
		}

		if iter > 10 && maxDelta < tol {
			return iter + 1, nil
		}
	}

	return iter, nil
}

func (s *SeepageSolver) getPermeability(i1, j1, i2, j2 int) float64 {
	perm1 := s.getPointPermeability(i1, j1)
	perm2 := s.getPointPermeability(i2, j2)
	return 2.0 * perm1 * perm2 / (perm1 + perm2)
}

func (s *SeepageSolver) getPointPermeability(i, j int) float64 {
	if !s.IsDamBody[j][i] {
		return s.PermeabilityK * 10
	}

	if s.Blanket.Enabled {
		x := s.XCoords[i]
		y := s.YCoords[j]
		if y > 0 && y <= s.Blanket.Thickness && x <= s.Blanket.Length {
			return s.Blanket.Permeability
		}
	}

	return s.PermeabilityK
}

func (s *SeepageSolver) CalculateVelocities() {
	dx := (s.Geometry.Length) / float64(s.GridNX-1)
	dy := (s.Geometry.Height + s.Geometry.FoundationDepth) / float64(s.GridNY-1)

	for j := 1; j < s.GridNY-1; j++ {
		for i := 1; i < s.GridNX-1; i++ {
			if !s.IsInDomain[j][i] {
				continue
			}

			k := s.getPointPermeability(i, j)

			dhdx := 0.0
			if s.IsInDomain[j][i+1] && s.IsInDomain[j][i-1] {
				dhdx = (s.WaterHead[j][i+1] - s.WaterHead[j][i-1]) / (2 * dx)
			} else if s.IsInDomain[j][i+1] {
				dhdx = (s.WaterHead[j][i+1] - s.WaterHead[j][i]) / dx
			} else if s.IsInDomain[j][i-1] {
				dhdx = (s.WaterHead[j][i] - s.WaterHead[j][i-1]) / dx
			}

			dhdy := 0.0
			if s.IsInDomain[j+1][i] && s.IsInDomain[j-1][i] {
				dhdy = (s.WaterHead[j+1][i] - s.WaterHead[j-1][i]) / (2 * dy)
			} else if s.IsInDomain[j+1][i] {
				dhdy = (s.WaterHead[j+1][i] - s.WaterHead[j][i]) / dy
			} else if s.IsInDomain[j-1][i] {
				dhdy = (s.WaterHead[j][i] - s.WaterHead[j-1][i]) / dy
			}

			s.VelocityX[j][i] = -k * dhdx
			s.VelocityY[j][i] = -k * dhdy

			y := s.YCoords[j]
			if s.WaterHead[j][i] > y {
				s.PorePressure[j][i] = (s.WaterHead[j][i] - y) * 9.81
			} else {
				s.PorePressure[j][i] = 0
			}
		}
	}
}

func (s *SeepageSolver) CalculateSeepageFlow() float64 {
	dy := (s.Geometry.Height + s.Geometry.FoundationDepth) / float64(s.GridNY-1)
	totalFlow := 0.0

	for j := 0; j < s.GridNY; j++ {
		for i := 0; i < s.GridNX; i++ {
			if s.IsInDomain[j][i] {
				x := s.XCoords[i]
				if x >= s.XCoords[s.GridNX-2] {
					totalFlow += s.VelocityX[j][i] * dy
				}
			}
		}
	}

	if totalFlow < 0 {
		totalFlow = -totalFlow
	}

	return totalFlow
}

func (s *SeepageSolver) GetMaxPorePressure() float64 {
	maxPP := 0.0
	for j := 0; j < s.GridNY; j++ {
		for i := 0; i < s.GridNX; i++ {
			if s.PorePressure[j][i] > maxPP {
				maxPP = s.PorePressure[j][i]
			}
		}
	}
	return maxPP
}

func (s *SeepageSolver) GetInfiltrationLine() []map[string]float64 {
	var line []map[string]float64
	for i := 0; i < s.GridNX; i++ {
		x := s.XCoords[i]
		var phreaticY float64 = -999

		for j := s.GridNY - 1; j >= 0; j-- {
			if s.IsInDomain[j][i] && s.WaterHead[j][i] > s.YCoords[j] {
				y := s.YCoords[j]
				h := s.WaterHead[j][i]
				phreaticY = y + (h - y)*0.5
				if j < s.GridNY-1 && s.IsInDomain[j+1][i] {
					y1 := s.YCoords[j]
					y2 := s.YCoords[j+1]
					h1 := s.WaterHead[j][i] - y1
					h2 := s.WaterHead[j+1][i] - y2
					if h2 < 0 && h1 > 0 {
						frac := h1 / (h1 - h2)
						phreaticY = y1 + frac*(y2-y1)
					}
				}
				break
			}
		}

		if phreaticY > -999 {
			line = append(line, map[string]float64{"x": x, "y": phreaticY})
		}
	}
	return line
}

func (s *SeepageSolver) RunSimulation(req models.SimulationRequest) (*models.SeepageSimulation, []models.SimulationGrid, error) {
	startTime := time.Now()

	s.SetWaterLevels(req.UpstreamWaterLevel, req.DownstreamWaterLevel)

	if req.GridResolutionX > 0 {
		s.GridNX = req.GridResolutionX
	}
	if req.GridResolutionY > 0 {
		s.GridNY = req.GridResolutionY
	}
	if req.PermeabilityK > 0 {
		s.PermeabilityK = req.PermeabilityK
	}
	if req.BlanketLength != nil && req.BlanketThickness != nil {
		s.SetBlanket(*req.BlanketLength, *req.BlanketThickness, s.PermeabilityK*0.01)
	}

	s.InitializeGrid()
	s.SetBoundaryConditions()

	maxIter := 500
	tol := 1e-5
	iter, err := s.SolveSteady(maxIter, tol)
	if err != nil {
		return nil, nil, fmt.Errorf("solver error: %w", err)
	}

	s.CalculateVelocities()

	seepageFlow := s.CalculateSeepageFlow()
	maxPorePressure := s.GetMaxPorePressure()
	infiltrationLine := s.GetInfiltrationLine()

	var grids []models.SimulationGrid
	gridCount := 0
	for j := 0; j < s.GridNY; j++ {
		for i := 0; i < s.GridNX; i++ {
			if !s.IsInDomain[j][i] {
				continue
			}
			vmag := math.Sqrt(s.VelocityX[j][i]*s.VelocityX[j][i] + s.VelocityY[j][i]*s.VelocityY[j][i])
			grids = append(grids, models.SimulationGrid{
				GridX:             s.XCoords[i],
				GridY:             s.YCoords[j],
				WaterHead:         s.WaterHead[j][i],
				PorePressure:      s.PorePressure[j][i],
				VelocityX:         s.VelocityX[j][i],
				VelocityY:         s.VelocityY[j][i],
				VelocityMagnitude: vmag,
				IsSaturated:       s.WaterHead[j][i] > s.YCoords[j],
			})
			gridCount++
		}
	}

	calcTime := time.Since(startTime).Milliseconds()

	simResult := &models.SeepageSimulation{
		SimulationName:      req.SimulationName,
		UpstreamWaterLevel:  req.UpstreamWaterLevel,
		DownstreamWaterLevel: req.DownstreamWaterLevel,
		TotalSeepageFlow:    seepageFlow,
		MaxPorePressure:     maxPorePressure,
		GridCount:           gridCount,
		CalculationTimeMs:   calcTime,
		Parameters: map[string]interface{}{
			"grid_nx":           s.GridNX,
			"grid_ny":           s.GridNY,
			"permeability_k":    s.PermeabilityK,
			"solver_iterations": iter,
			"blanket_enabled":   s.Blanket.Enabled,
			"blanket_length":    s.Blanket.Length,
			"blanket_thickness": s.Blanket.Thickness,
		},
		ResultSummary: map[string]interface{}{
			"seepage_flow_lps":       seepageFlow * 1000,
			"max_pore_pressure_kpa":  maxPorePressure,
			"infiltration_line":      infiltrationLine,
			"upstream_gradient":      (s.UpstreamH - s.DownstreamH) / s.Geometry.Length,
		},
	}

	return simResult, grids, nil
}
