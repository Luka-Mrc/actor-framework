package crdt

import (
	"encoding/json"
	"fmt"
	"sort"
)

type ORSet struct {
	node    string
	seq     uint64
	adds    map[string]map[string]bool
	removes map[string]map[string]bool
}

func NewORSet(node string) *ORSet {
	return &ORSet{
		node:    node,
		adds:    make(map[string]map[string]bool),
		removes: make(map[string]map[string]bool),
	}
}

func (s *ORSet) Add(elem string) {
	s.seq++
	tag := fmt.Sprintf("%s#%d", s.node, s.seq)
	if s.adds[elem] == nil {
		s.adds[elem] = make(map[string]bool)
	}
	s.adds[elem][tag] = true
}

func (s *ORSet) Remove(elem string) {
	for tag := range s.adds[elem] {
		if s.removes[elem] == nil {
			s.removes[elem] = make(map[string]bool)
		}
		s.removes[elem][tag] = true
	}
}

func (s *ORSet) Contains(elem string) bool {
	for tag := range s.adds[elem] {
		if !s.removes[elem][tag] {
			return true
		}
	}
	return false
}

func (s *ORSet) Elements() []string {
	out := []string{}
	for elem := range s.adds {
		if s.Contains(elem) {
			out = append(out, elem)
		}
	}
	sort.Strings(out)
	return out
}

func (s *ORSet) Merge(other *ORSet) {
	if other == nil {
		return
	}
	mergeTags(s.adds, other.adds)
	mergeTags(s.removes, other.removes)
}

func mergeTags(dst, src map[string]map[string]bool) {
	for elem, tags := range src {
		if dst[elem] == nil {
			dst[elem] = make(map[string]bool)
		}
		for tag := range tags {
			dst[elem][tag] = true
		}
	}
}

type orsetDTO struct {
	Adds    map[string]map[string]bool `json:"adds"`
	Removes map[string]map[string]bool `json:"removes"`
}

func (s *ORSet) Marshal() []byte {
	b, _ := json.Marshal(orsetDTO{Adds: s.adds, Removes: s.removes})
	return b
}

func UnmarshalORSet(b []byte) (*ORSet, error) {
	var dto orsetDTO
	if len(b) > 0 {
		if err := json.Unmarshal(b, &dto); err != nil {
			return nil, err
		}
	}
	if dto.Adds == nil {
		dto.Adds = make(map[string]map[string]bool)
	}
	if dto.Removes == nil {
		dto.Removes = make(map[string]map[string]bool)
	}
	return &ORSet{adds: dto.Adds, removes: dto.Removes}, nil
}
