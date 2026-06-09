package model

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
)

type Weights struct {
	InputDim  int
	Hidden1   int
	Hidden2   int
	OutputDim int

	W1 [][]float64
	B1 []float64
	W2 [][]float64
	B2 []float64
	W3 [][]float64
	B3 []float64
}

func newMatrix(rows, cols int) [][]float64 {
	m := make([][]float64, rows)
	for i := range m {
		m[i] = make([]float64, cols)
	}
	return m
}

func heInit(m [][]float64, rng *rand.Rand) {
	if len(m) == 0 {
		return
	}
	std := math.Sqrt(2.0 / float64(len(m[0])))
	for i := range m {
		for j := range m[i] {
			m[i][j] = rng.NormFloat64() * std
		}
	}
}

func NewWeights(inputDim, hidden1, hidden2, outputDim int, seed int64) *Weights {
	rng := rand.New(rand.NewSource(seed))
	w := &Weights{
		InputDim:  inputDim,
		Hidden1:   hidden1,
		Hidden2:   hidden2,
		OutputDim: outputDim,
		W1:        newMatrix(hidden1, inputDim),
		B1:        make([]float64, hidden1),
		W2:        newMatrix(hidden2, hidden1),
		B2:        make([]float64, hidden2),
		W3:        newMatrix(outputDim, hidden2),
		B3:        make([]float64, outputDim),
	}
	heInit(w.W1, rng)
	heInit(w.W2, rng)
	heInit(w.W3, rng)
	return w
}

func (w *Weights) Marshal() []byte {
	var buf bytes.Buffer
	writeInt := func(n int) { _ = binary.Write(&buf, binary.LittleEndian, int32(n)) }
	writeMat := func(m [][]float64) {
		for i := range m {
			_ = binary.Write(&buf, binary.LittleEndian, m[i])
		}
	}
	writeInt(w.InputDim)
	writeInt(w.Hidden1)
	writeInt(w.Hidden2)
	writeInt(w.OutputDim)
	writeMat(w.W1)
	_ = binary.Write(&buf, binary.LittleEndian, w.B1)
	writeMat(w.W2)
	_ = binary.Write(&buf, binary.LittleEndian, w.B2)
	writeMat(w.W3)
	_ = binary.Write(&buf, binary.LittleEndian, w.B3)
	return buf.Bytes()
}

func UnmarshalWeights(data []byte) (*Weights, error) {
	r := bytes.NewReader(data)
	var readInt = func() (int, error) {
		var n int32
		err := binary.Read(r, binary.LittleEndian, &n)
		return int(n), err
	}
	dims := make([]int, 4)
	for i := range dims {
		v, err := readInt()
		if err != nil {
			return nil, fmt.Errorf("model: čitanje dimenzija: %w", err)
		}
		dims[i] = v
	}
	w := NewWeights(dims[0], dims[1], dims[2], dims[3], 1)

	readMat := func(m [][]float64) error {
		for i := range m {
			if err := binary.Read(r, binary.LittleEndian, m[i]); err != nil {
				return err
			}
		}
		return nil
	}
	if err := readMat(w.W1); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, w.B1); err != nil {
		return nil, err
	}
	if err := readMat(w.W2); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, w.B2); err != nil {
		return nil, err
	}
	if err := readMat(w.W3); err != nil {
		return nil, err
	}
	if err := binary.Read(r, binary.LittleEndian, w.B3); err != nil {
		return nil, err
	}
	return w, nil
}
