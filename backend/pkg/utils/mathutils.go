package utils

import (
	"math"
	"math/rand"
	"time"
)

func RoundFloat(val float64, precision int) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

func Linspace(start, end float64, num int) []float64 {
	if num <= 0 {
		return []float64{}
	}
	if num == 1 {
		return []float64{start}
	}
	result := make([]float64, num)
	step := (end - start) / float64(num-1)
	for i := 0; i < num; i++ {
		result[i] = start + step*float64(i)
	}
	return result
}

func Meshgrid(x, y []float64) ([][]float64, [][]float64) {
	X := make([][]float64, len(y))
	Y := make([][]float64, len(y))
	for i := range y {
		X[i] = make([]float64, len(x))
		Y[i] = make([]float64, len(x))
		for j := range x {
			X[i][j] = x[j]
			Y[i][j] = y[i]
		}
	}
	return X, Y
}

func GaussSeidel(A [][]float64, b []float64, x0 []float64, tol float64, maxIter int) ([]float64, int, error) {
	n := len(b)
	x := make([]float64, n)
	copy(x, x0)

	for iter := 0; iter < maxIter; iter++ {
		maxDiff := 0.0
		for i := 0; i < n; i++ {
			sum := 0.0
			for j := 0; j < n; j++ {
				if j != i {
					sum += A[i][j] * x[j]
				}
			}
			newXi := (b[i] - sum) / A[i][i]
			diff := math.Abs(newXi - x[i])
			if diff > maxDiff {
				maxDiff = diff
			}
			x[i] = newXi
		}
		if maxDiff < tol {
			return x, iter + 1, nil
		}
	}
	return x, maxIter, nil
}

func Norm2(v []float64) float64 {
	sum := 0.0
	for _, val := range v {
		sum += val * val
	}
	return math.Sqrt(sum)
}

func SolveTriDiagonal(a, b, c, d []float64) []float64 {
	n := len(d)
	cp := make([]float64, n)
	dp := make([]float64, n)
	x := make([]float64, n)

	cp[0] = c[0] / b[0]
	dp[0] = d[0] / b[0]

	for i := 1; i < n; i++ {
		m := b[i] - a[i]*cp[i-1]
		cp[i] = c[i] / m
		dp[i] = (d[i] - a[i]*dp[i-1]) / m
	}

	x[n-1] = dp[n-1]
	for i := n - 2; i >= 0; i-- {
		x[i] = dp[i] - cp[i]*x[i+1]
	}

	return x
}

func NewRand(seed ...int64) *rand.Rand {
	var s int64
	if len(seed) > 0 {
		s = seed[0]
	} else {
		s = time.Now().UnixNano()
	}
	return rand.New(rand.NewSource(s))
}

func Clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func Max(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func Min(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values[1:] {
		if v < m {
			m = v
		}
	}
	return m
}
