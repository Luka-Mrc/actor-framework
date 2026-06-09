package model

type Metrics struct {
	Accuracy  float64
	Precision []float64
	Recall    []float64
	F1        []float64
	MacroF1   float64
	Confusion [][]int
	Total     int
}

func Evaluate(w *Weights, X [][]float64, Y []int) Metrics {
	n := w.OutputDim
	conf := make([][]int, n)
	for i := range conf {
		conf[i] = make([]int, n)
	}

	correct := 0
	for i := range X {
		pred := Predict(w, X[i])
		actual := Y[i]
		conf[actual][pred]++
		if pred == actual {
			correct++
		}
	}

	m := Metrics{
		Precision: make([]float64, n),
		Recall:    make([]float64, n),
		F1:        make([]float64, n),
		Confusion: conf,
		Total:     len(X),
	}
	if len(X) > 0 {
		m.Accuracy = float64(correct) / float64(len(X))
	}

	var macro float64
	for c := 0; c < n; c++ {
		tp := conf[c][c]
		var fp, fn int
		for k := 0; k < n; k++ {
			if k != c {
				fp += conf[k][c]
				fn += conf[c][k]
			}
		}
		if tp+fp > 0 {
			m.Precision[c] = float64(tp) / float64(tp+fp)
		}
		if tp+fn > 0 {
			m.Recall[c] = float64(tp) / float64(tp+fn)
		}
		if m.Precision[c]+m.Recall[c] > 0 {
			m.F1[c] = 2 * m.Precision[c] * m.Recall[c] / (m.Precision[c] + m.Recall[c])
		}
		macro += m.F1[c]
	}
	if n > 0 {
		m.MacroF1 = macro / float64(n)
	}
	return m
}
