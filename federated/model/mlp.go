package model

import "math"

func relu(x float64) float64 {
	if x > 0 {
		return x
	}
	return 0
}

func Softmax(logits []float64) []float64 {
	maxLogit := math.Inf(-1)
	for _, v := range logits {
		if v > maxLogit {
			maxLogit = v
		}
	}
	out := make([]float64, len(logits))
	var sum float64
	for i, v := range logits {
		out[i] = math.Exp(v - maxLogit)
		sum += out[i]
	}
	for i := range out {
		out[i] /= sum
	}
	return out
}

type Cache struct {
	X     []float64
	Z1    []float64
	A1    []float64
	Z2    []float64
	A2    []float64
	Probs []float64
}

func linear(m [][]float64, b, v []float64) []float64 {
	out := make([]float64, len(m))
	for j := range m {
		s := b[j]
		row := m[j]
		for k := range row {
			s += row[k] * v[k]
		}
		out[j] = s
	}
	return out
}

func reluVec(z []float64) []float64 {
	a := make([]float64, len(z))
	for i, v := range z {
		a[i] = relu(v)
	}
	return a
}

func Forward(w *Weights, x []float64) *Cache {
	z1 := linear(w.W1, w.B1, x)
	a1 := reluVec(z1)
	z2 := linear(w.W2, w.B2, a1)
	a2 := reluVec(z2)
	z3 := linear(w.W3, w.B3, a2)
	probs := Softmax(z3)
	return &Cache{X: x, Z1: z1, A1: a1, Z2: z2, A2: a2, Probs: probs}
}

func Predict(w *Weights, x []float64) int {
	probs := Forward(w, x).Probs
	best, bestIdx := probs[0], 0
	for i, p := range probs {
		if p > best {
			best, bestIdx = p, i
		}
	}
	return bestIdx
}
