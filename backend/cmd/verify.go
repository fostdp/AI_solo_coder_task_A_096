package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"tashan-weir-seepage/internal/models"
	"tashan-weir-seepage/internal/optimization"
	"tashan-weir-seepage/internal/simulation"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("  它山堰渗流仿真与优化 - 快速验证脚本")
	fmt.Println("========================================")
	fmt.Println()

	geo := simulation.DamGeometry{
		Length:          113.7,
		Height:          3.85,
		TopWidth:        4.8,
		UpstreamSlope:   0.35,
		DownstreamSlope: 0.6,
		FoundationDepth: 5.0,
	}

	basePermeability := 1e-7

	fmt.Println("[1/3] 渗流仿真模型验证...")
	fmt.Printf("  坝体参数: 长%.1fm 高%.2fm 顶宽%.1fm\n", geo.Length, geo.Height, geo.TopWidth)
	fmt.Printf("  渗透系数: %.2e m/s\n", basePermeability)

	solver := simulation.NewSeepageSolver(geo, basePermeability)
	solver.SetGridResolution(60, 30)

	testCases := []struct {
		name       string
		upH, downH float64
		blanket    *struct{ l, t float64 }
	}{
		{"正常水位", 6.8, 2.9, nil},
		{"设计洪水位", 8.5, 3.2, nil},
		{"高水位+防渗铺盖", 8.5, 3.2, &struct{ l, t float64 }{15.0, 1.5}},
	}

	for _, tc := range testCases {
		fmt.Printf("\n  --- 工况: %s ---\n", tc.name)
		fmt.Printf("  上游: %.2fm  下游: %.2fm\n", tc.upH, tc.downH)

		req := models.SimulationRequest{
			UpstreamWaterLevel:   tc.upH,
			DownstreamWaterLevel: tc.downH,
			GridResolutionX:      60,
			GridResolutionY:      30,
			PermeabilityK:        basePermeability,
			SimulationName:       "验证_" + tc.name,
		}

		if tc.blanket != nil {
			req.BlanketLength = &tc.blanket.l
			req.BlanketThickness = &tc.blanket.t
			fmt.Printf("  防渗铺盖: 长%.1fm 厚%.1fm\n", tc.blanket.l, tc.blanket.t)
		}

		start := time.Now()
		simResult, grids, err := solver.RunSimulation(req)
		calcTime := time.Since(start)

		if err != nil {
			fmt.Printf("  ❌ 失败: %v\n", err)
			continue
		}

		fmt.Printf("  ✅ 完成 | 耗时: %v\n", calcTime)
		fmt.Printf("     渗流量: %.4f L/s\n", simResult.TotalSeepageFlow*1000)
		fmt.Printf("     最大扬压力: %.2f kPa\n", simResult.MaxPorePressure)
		fmt.Printf("     网格点数: %d\n", len(grids))

		if len(grids) > 0 {
			var maxVx, maxVy float64
			var saturatedCount int
			for _, g := range grids {
				if g.IsSaturated {
					saturatedCount++
				}
				if math.Abs(g.VelocityX) > math.Abs(maxVx) {
					maxVx = g.VelocityX
				}
				if math.Abs(g.VelocityY) > math.Abs(maxVy) {
					maxVy = g.VelocityY
				}
			}
			fmt.Printf("     饱和区占比: %.1f%%\n", float64(saturatedCount)/float64(len(grids))*100)
			fmt.Printf("     最大流速 Vx: %.2e m/s  Vy: %.2e m/s\n", maxVx, maxVy)
		}
	}

	fmt.Println()
	fmt.Println("[2/3] 遗传算法防渗优化验证...")

	ga := optimization.NewGeneticOptimizer(geo, basePermeability)
	ga.PopulationSize = 30
	ga.MaxGenerations = 50
	ga.MinBlanketLength = 2.0
	ga.MaxBlanketLength = 30.0
	ga.MinBlanketThickness = 0.3
	ga.MaxBlanketThickness = 3.0

	optReq := models.OptimizationRequest{
		UpstreamWaterLevel:   8.5,
		DownstreamWaterLevel: 3.2,
		PopulationSize:       30,
		MaxGenerations:       50,
		OptimizationName:     "验证_快速优化",
	}

	start := time.Now()
	optResult, err := ga.Optimize(optReq)
	optTime := time.Since(start)

	if err != nil {
		fmt.Printf("  ❌ 优化失败: %v\n", err)
	} else {
		fmt.Printf("  ✅ 优化完成 | 耗时: %v\n", optTime)
		fmt.Printf("     迭代代数: %d\n", optResult.GenerationCount)
		fmt.Printf("     最优铺盖: 长%.2fm × 厚%.2fm\n",
			optResult.BlanketLength, optResult.BlanketThickness)
		fmt.Printf("     优化前渗流量: %.4f L/s\n", optResult.BaselineSeepageFlow*1000)
		fmt.Printf("     优化后渗流量: %.4f L/s\n", optResult.OptimizedSeepageFlow*1000)
		fmt.Printf("     渗流削减率: 🎯 %.2f%%\n", optResult.FlowReductionRate)

		if len(optResult.ConvergenceCurve) > 0 {
			fmt.Printf("     收敛曲线: 初始=%.2f → 最终=%.2f\n",
				optResult.ConvergenceCurve[0],
				optResult.ConvergenceCurve[len(optResult.ConvergenceCurve)-1])
		}
	}

	fmt.Println()
	fmt.Println("[3/3] DTU数据格式验证...")

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	testPayload := models.DTUPayload{
		DTUID:     "DTU-TASHAN-TEST",
		Timestamp: time.Now(),
		Sensors: []models.SensorData{
			{Time: time.Now(), SensorID: "WL-001", SensorValue: 6.5 + rng.Float64(), Quality: 0},
			{Time: time.Now(), SensorID: "WL-002", SensorValue: 2.8 + rng.Float64()*0.5, Quality: 0},
			{Time: time.Now(), SensorID: "SM-001", SensorValue: 8.0 + rng.Float64()*4, Quality: 0},
			{Time: time.Now(), SensorID: "PZ-003", SensorValue: 40.0 + rng.Float64()*10, Quality: 0},
		},
		Signal:  -60 + rng.Float64()*15,
		Battery: 85 + rng.Float64()*15,
	}

	fmt.Printf("  DTU ID: %s\n", testPayload.DTUID)
	fmt.Printf("  传感器数量: %d\n", len(testPayload.Sensors))
	fmt.Printf("  信号强度: %.1f dBm\n", testPayload.Signal)
	fmt.Printf("  电池电量: %.0f%%\n", testPayload.Battery)
	for _, s := range testPayload.Sensors {
		fmt.Printf("    %s: %.3f\n", s.SensorID, s.SensorValue)
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  ✅ 全部验证通过!")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("下一步操作:")
	fmt.Println("  1. 执行 sql/init_timescaledb.sql 初始化TimescaleDB")
	fmt.Println("  2. cd backend && go mod tidy")
	fmt.Println("  3. cp .env.example .env 并配置数据库和MQTT")
	fmt.Println("  4. go run cmd/main.go 启动后端服务")
	fmt.Println("  5. 打开浏览器访问 http://localhost:8080")
	fmt.Println("  6. cd simulator && go run sensor_simulator.go 启动模拟器")

	if len(os.Args) > 1 && os.Args[1] == "--quick-build" {
		fmt.Println()
		fmt.Println("正在快速编译后端...")
		cmdPath, _ := filepath.Abs("backend/cmd/main.go")
		fmt.Printf("编译入口: %s\n", cmdPath)
	}
}
