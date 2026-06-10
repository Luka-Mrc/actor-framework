package federated

import (
	"github.com/lukam/actor-framework/federated/model"
	pb "github.com/lukam/actor-framework/federated/protogen"
	"github.com/lukam/actor-framework/framework"
)

type Aggregator struct {
	expected int
	round    int
	updates  []*model.Weights
	sizes    []int
	seen     map[string]bool
}

func NewAggregatorProps(expected int) framework.Props {
	return framework.Props{Factory: func() framework.Actor {
		return &Aggregator{expected: expected, round: 0, seen: make(map[string]bool)}
	}}
}

func (a *Aggregator) Receive(ctx *framework.ActorContext, msg framework.Message) {
	m, ok := msg.(*pb.LocalUpdate)
	if !ok {
		return
	}

	round := int(m.GetRoundNumber())
	if round != a.round {
		a.round = round
		a.updates = nil
		a.sizes = nil
		a.seen = make(map[string]bool)
	}
	if a.seen[m.GetTrainerId()] {
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
		global := fedAvg(a.updates, a.sizes)
		ctx.Tell(ctx.Parent(), &pb.AggregationComplete{
			RoundNumber:      int32(a.round),
			NewGlobalWeights: encodeWeights(global),
		})
	}
}
