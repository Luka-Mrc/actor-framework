package federated

import (
	"github.com/lukam/actor-framework/federated/model"
	pb "github.com/lukam/actor-framework/federated/protogen"
	"github.com/lukam/actor-framework/framework"
)

type finalizeRound struct{ round int }

type Aggregator struct {
	expected  int
	round     int
	updates   []*model.Weights
	sizes     []int
	seen      map[string]bool
	completed bool
}

func NewAggregatorProps(expected int) framework.Props {
	return framework.Props{Factory: func() framework.Actor {
		return &Aggregator{expected: expected, round: 0, seen: make(map[string]bool)}
	}}
}

func (a *Aggregator) Receive(ctx *framework.ActorContext, msg framework.Message) {
	switch m := msg.(type) {
	case *pb.LocalUpdate:
		round := int(m.GetRoundNumber())
		a.resetIfNewRound(round)
		if a.completed || a.seen[m.GetTrainerId()] {
			return
		}
		w, err := decodeWeights(m.GetUpdatedWeights())
		if err != nil {
			ctx.System().Logger().Error("aggregator: decode weights", "error", err.Error())
			return
		}
		a.seen[m.GetTrainerId()] = true
		a.updates = append(a.updates, w)
		a.sizes = append(a.sizes, int(m.GetDatasetSize()))
		if len(a.updates) == a.expected {
			a.finalize(ctx)
		}

	case finalizeRound:
		if m.round != a.round || a.completed || len(a.updates) == 0 {
			return
		}
		ctx.System().Logger().Warn("aggregator finalizing early",
			"round", a.round, "received", len(a.updates), "expected", a.expected)
		a.finalize(ctx)
	}
}

func (a *Aggregator) resetIfNewRound(round int) {
	if round != a.round {
		a.round = round
		a.updates = nil
		a.sizes = nil
		a.seen = make(map[string]bool)
		a.completed = false
	}
}

func (a *Aggregator) finalize(ctx *framework.ActorContext) {
	global := fedAvg(a.updates, a.sizes)
	a.completed = true
	ctx.Tell(ctx.Parent(), &pb.AggregationComplete{
		RoundNumber:      int32(a.round),
		NewGlobalWeights: encodeWeights(global),
	})
}
