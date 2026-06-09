package data

import "math/rand"

func newShard(d *Dataset, idx []int) *Dataset {
	s := &Dataset{Schema: d.Schema, X: make([][]float64, len(idx)), Y: make([]int, len(idx))}
	for i, j := range idx {
		s.X[i] = d.X[j]
		s.Y[i] = d.Y[j]
	}
	return s
}

func SplitIID(d *Dataset, n int, seed int64) []*Dataset {
	rng := rand.New(rand.NewSource(seed))
	perm := rng.Perm(d.Len())
	shards := make([]*Dataset, n)
	for i := 0; i < n; i++ {
		var idx []int
		for j := i; j < len(perm); j += n {
			idx = append(idx, perm[j])
		}
		shards[i] = newShard(d, idx)
	}
	return shards
}

func SplitNonIID(d *Dataset, n int, primaryFraction float64, seed int64) []*Dataset {
	rng := rand.New(rand.NewSource(seed))

	byClass := make([][]int, NumClasses)
	for i, y := range d.Y {
		byClass[y] = append(byClass[y], i)
	}
	for c := range byClass {
		rng.Shuffle(len(byClass[c]), func(a, b int) {
			byClass[c][a], byClass[c][b] = byClass[c][b], byClass[c][a]
		})
	}

	pos := make([]int, NumClasses)
	shardSize := d.Len() / n
	shards := make([]*Dataset, n)

	for i := 0; i < n; i++ {
		primary := i % NumClasses
		var idx []int

		nPrimary := int(float64(shardSize) * primaryFraction)
		for k := 0; k < nPrimary && pos[primary] < len(byClass[primary]); k++ {
			idx = append(idx, byClass[primary][pos[primary]])
			pos[primary]++
		}

		next := 0
		for len(idx) < shardSize {
			found := false
			for tries := 0; tries < NumClasses; tries++ {
				cl := next % NumClasses
				next++
				if cl == primary || pos[cl] >= len(byClass[cl]) {
					continue
				}
				idx = append(idx, byClass[cl][pos[cl]])
				pos[cl]++
				found = true
				break
			}
			if !found {
				break
			}
		}
		shards[i] = newShard(d, idx)
	}
	return shards
}
