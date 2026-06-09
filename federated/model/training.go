package model

import "math"

func CrossEntropy(probs []float64, label int) float64 {
	p := probs[label]
	if p < 1e-12 {
		p = 1e-12
	}
	return -math.Log(p)
}

type Gradients struct {
	W1 [][]float64
	B1 []float64
	W2 [][]float64
	B2 []float64
	W3 [][]float64
	B3 []float64
}

func newGradients(w *Weights) *Gradients {
	return &Gradients{
		W1: newMatrix(w.Hidden1, w.InputDim),
		B1: make([]float64, w.Hidden1),
		W2: newMatrix(w.Hidden2, w.Hidden1),
		B2: make([]float64, w.Hidden2),
		W3: newMatrix(w.OutputDim, w.Hidden2),
		B3: make([]float64, w.OutputDim),
	}
}

func reluGrad(z float64) float64 {
	if z > 0 {
		return 1
	}
	return 0
}

func Backward(w *Weights, c *Cache, label int) *Gradients {
	g := newGradients(w)

	dz3 := make([]float64, w.OutputDim)
	copy(dz3, c.Probs)
	dz3[label] -= 1
	for j := 0; j < w.OutputDim; j++ {
		g.B3[j] = dz3[j]
		for k := 0; k < w.Hidden2; k++ {
			g.W3[j][k] = dz3[j] * c.A2[k]
		}
	}

	dz2 := make([]float64, w.Hidden2)
	for k := 0; k < w.Hidden2; k++ {
		var da float64
		for j := 0; j < w.OutputDim; j++ {
			da += w.W3[j][k] * dz3[j]
		}
		dz2[k] = da * reluGrad(c.Z2[k])
	}
	for j := 0; j < w.Hidden2; j++ {
		g.B2[j] = dz2[j]
		for k := 0; k < w.Hidden1; k++ {
			g.W2[j][k] = dz2[j] * c.A1[k]
		}
	}

	dz1 := make([]float64, w.Hidden1)
	for k := 0; k < w.Hidden1; k++ {
		var da float64
		for j := 0; j < w.Hidden2; j++ {
			da += w.W2[j][k] * dz2[j]
		}
		dz1[k] = da * reluGrad(c.Z1[k])
	}
	for j := 0; j < w.Hidden1; j++ {
		g.B1[j] = dz1[j]
		for k := 0; k < w.InputDim; k++ {
			g.W1[j][k] = dz1[j] * c.X[k]
		}
	}

	return g
}

func stepMat(m, g [][]float64, lr float64) {
	for i := range m {
		for j := range m[i] {
			m[i][j] -= lr * g[i][j]
		}
	}
}

func stepVec(v, g []float64, lr float64) {
	for i := range v {
		v[i] -= lr * g[i]
	}
}

func (w *Weights) SGDStep(g *Gradients, lr float64) {
	stepMat(w.W1, g.W1, lr)
	stepVec(w.B1, g.B1, lr)
	stepMat(w.W2, g.W2, lr)
	stepVec(w.B2, g.B2, lr)
	stepMat(w.W3, g.W3, lr)
	stepVec(w.B3, g.B3, lr)
}

func TrainEpoch(w *Weights, X [][]float64, Y []int, lr float64) float64 {
	if len(X) == 0 {
		return 0
	}
	var total float64
	for i := range X {
		c := Forward(w, X[i])
		total += CrossEntropy(c.Probs, Y[i])
		g := Backward(w, c, Y[i])
		w.SGDStep(g, lr)
	}
	return total / float64(len(X))
}
