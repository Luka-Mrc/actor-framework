package crdt

import "encoding/json"

type GCounter struct {
	counts map[string]uint64
}

func NewGCounter() *GCounter {
	return &GCounter{counts: make(map[string]uint64)}
}

func (g *GCounter) Increment(node string, n uint64) {
	g.counts[node] += n
}

func (g *GCounter) Value() uint64 {
	var sum uint64
	for _, v := range g.counts {
		sum += v
	}
	return sum
}

func (g *GCounter) Merge(other *GCounter) {
	if other == nil {
		return
	}
	for node, v := range other.counts {
		if v > g.counts[node] {
			g.counts[node] = v
		}
	}
}

func (g *GCounter) Marshal() []byte {
	b, _ := json.Marshal(g.counts)
	return b
}

func UnmarshalGCounter(b []byte) (*GCounter, error) {
	counts := make(map[string]uint64)
	if len(b) > 0 {
		if err := json.Unmarshal(b, &counts); err != nil {
			return nil, err
		}
	}
	return &GCounter{counts: counts}, nil
}
