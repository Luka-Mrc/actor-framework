package federated

import "github.com/lukam/actor-framework/federated/model"

func encodeWeights(w *model.Weights) []byte { return w.Marshal() }

func decodeWeights(b []byte) (*model.Weights, error) { return model.UnmarshalWeights(b) }

func zeroMat(rows, cols int) [][]float64 {
	m := make([][]float64, rows)
	for i := range m {
		m[i] = make([]float64, cols)
	}
	return m
}

func zeroLike(w *model.Weights) *model.Weights {
	return &model.Weights{
		InputDim:  w.InputDim,
		Hidden1:   w.Hidden1,
		Hidden2:   w.Hidden2,
		OutputDim: w.OutputDim,
		W1:        zeroMat(w.Hidden1, w.InputDim),
		B1:        make([]float64, w.Hidden1),
		W2:        zeroMat(w.Hidden2, w.Hidden1),
		B2:        make([]float64, w.Hidden2),
		W3:        zeroMat(w.OutputDim, w.Hidden2),
		B3:        make([]float64, w.OutputDim),
	}
}

func addScaledMat(dst, src [][]float64, w float64) {
	for i := range dst {
		for j := range dst[i] {
			dst[i][j] += w * src[i][j]
		}
	}
}

func addScaledVec(dst, src []float64, w float64) {
	for i := range dst {
		dst[i] += w * src[i]
	}
}

func fedAvg(updates []*model.Weights, sizes []int) *model.Weights {
	acc := zeroLike(updates[0])

	var total float64
	for _, s := range sizes {
		total += float64(s)
	}
	if total == 0 {
		total = 1
	}

	for u, w := range updates {
		frac := float64(sizes[u]) / total
		addScaledMat(acc.W1, w.W1, frac)
		addScaledVec(acc.B1, w.B1, frac)
		addScaledMat(acc.W2, w.W2, frac)
		addScaledVec(acc.B2, w.B2, frac)
		addScaledMat(acc.W3, w.W3, frac)
		addScaledVec(acc.B3, w.B3, frac)
	}
	return acc
}
