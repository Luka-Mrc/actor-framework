package data

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

const numFeatures = 41

var catCols = [3]int{1, 2, 3}

type rawRecord struct {
	fields [numFeatures]string
	label  string
}

type categorical struct {
	values map[string]int
	size   int
}

type Schema struct {
	isCat    [numFeatures]bool
	catIndex [numFeatures]int
	cats     [3]categorical
	min      [numFeatures]float64
	max      [numFeatures]float64
	InputDim int
}

func newSchema() *Schema {
	s := &Schema{}
	for i, col := range catCols {
		s.isCat[col] = true
		s.catIndex[col] = i
		s.cats[i].values = make(map[string]int)
	}
	for c := 0; c < numFeatures; c++ {
		s.min[c] = math.Inf(1)
		s.max[c] = math.Inf(-1)
	}
	return s
}

func (s *Schema) fit(records []rawRecord) {
	for _, r := range records {
		for c := 0; c < numFeatures; c++ {
			if s.isCat[c] {
				cat := &s.cats[s.catIndex[c]]
				if _, ok := cat.values[r.fields[c]]; !ok {
					cat.values[r.fields[c]] = cat.size
					cat.size++
				}
			} else {
				x, err := strconv.ParseFloat(r.fields[c], 64)
				if err != nil {
					continue
				}
				if x < s.min[c] {
					s.min[c] = x
				}
				if x > s.max[c] {
					s.max[c] = x
				}
			}
		}
	}
	dim := 0
	for c := 0; c < numFeatures; c++ {
		if s.isCat[c] {
			dim += s.cats[s.catIndex[c]].size
		} else {
			dim++
		}
	}
	s.InputDim = dim
}

func (s *Schema) scale(c int, x float64) float64 {
	rng := s.max[c] - s.min[c]
	if rng == 0 {
		return 0
	}
	v := (x - s.min[c]) / rng
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func (s *Schema) encode(r rawRecord) []float64 {
	out := make([]float64, 0, s.InputDim)
	for c := 0; c < numFeatures; c++ {
		if s.isCat[c] {
			cat := s.cats[s.catIndex[c]]
			block := make([]float64, cat.size)
			if pos, ok := cat.values[r.fields[c]]; ok {
				block[pos] = 1
			}
			out = append(out, block...)
		} else {
			x, _ := strconv.ParseFloat(r.fields[c], 64)
			out = append(out, s.scale(c, x))
		}
	}
	return out
}

type Dataset struct {
	X      [][]float64
	Y      []int
	Schema *Schema
}

func (d *Dataset) Len() int      { return len(d.X) }
func (d *Dataset) InputDim() int { return d.Schema.InputDim }

func readRecords(path string) (records []rawRecord, skipped int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < numFeatures+1 {
			skipped++
			continue
		}
		var r rawRecord
		copy(r.fields[:], parts[:numFeatures])
		r.label = parts[numFeatures]
		if _, ok := ClassOf(r.label); !ok {
			skipped++
			continue
		}
		records = append(records, r)
	}
	if err := sc.Err(); err != nil {
		return nil, skipped, err
	}
	return records, skipped, nil
}

func encodeAll(records []rawRecord, s *Schema) *Dataset {
	d := &Dataset{Schema: s, X: make([][]float64, 0, len(records)), Y: make([]int, 0, len(records))}
	for _, r := range records {
		c, _ := ClassOf(r.label)
		d.X = append(d.X, s.encode(r))
		d.Y = append(d.Y, int(c))
	}
	return d
}

func LoadTrain(path string) (*Dataset, error) {
	records, _, err := readRecords(path)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("data: prazan train skup %q", path)
	}
	s := newSchema()
	s.fit(records)
	return encodeAll(records, s), nil
}

func LoadTest(path string, s *Schema) (*Dataset, error) {
	if s == nil {
		return nil, fmt.Errorf("data: Schema je nil (prvo pozovi LoadTrain)")
	}
	records, _, err := readRecords(path)
	if err != nil {
		return nil, err
	}
	return encodeAll(records, s), nil
}
