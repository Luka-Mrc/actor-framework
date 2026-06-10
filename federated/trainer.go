package federated

import (
	"github.com/lukam/actor-framework/federated/data"
	"github.com/lukam/actor-framework/federated/model"
	pb "github.com/lukam/actor-framework/federated/protogen"
	"github.com/lukam/actor-framework/framework"
)

type Trainer struct {
	id        string
	coordAddr string
	local     *data.Dataset
	epochs    int
	lr        float64

	lastRound   int
	lastWeights *model.Weights
	lastLoss    float64
}

func NewTrainerProps(id, coordAddr string, local *data.Dataset, epochs int, lr float64) framework.Props {
	return framework.Props{Factory: func() framework.Actor {
		return &Trainer{id: id, coordAddr: coordAddr, local: local, epochs: epochs, lr: lr, lastRound: -1}
	}}
}

func (t *Trainer) PreStart(ctx *framework.ActorContext) {
	coord := ctx.System().Resolve(t.coordAddr)
	ctx.Tell(coord, &pb.RegisterTrainer{
		TrainerId:   t.id,
		DatasetSize: int32(t.local.Len()),
		Address:     ctx.Self().Address(),
	})
}

func (t *Trainer) Receive(ctx *framework.ActorContext, msg framework.Message) {
	m, ok := msg.(*pb.StartRound)
	if !ok {
		return
	}
	round := int(m.GetRoundNumber())
	agg := ctx.System().Resolve(m.GetAggregatorAddress())

	if round == t.lastRound && t.lastWeights != nil {
		ctx.Tell(agg, t.localUpdate(round))
		return
	}

	w, err := decodeWeights(m.GetGlobalWeights())
	if err != nil {
		ctx.System().Logger().Error("trainer: decode weights", "trainer", t.id, "error", err.Error())
		return
	}
	var loss float64
	for e := 0; e < t.epochs; e++ {
		loss = model.TrainEpoch(w, t.local.X, t.local.Y, t.lr)
	}
	t.lastRound = round
	t.lastWeights = w
	t.lastLoss = loss
	ctx.Tell(agg, t.localUpdate(round))
}

func (t *Trainer) localUpdate(round int) *pb.LocalUpdate {
	return &pb.LocalUpdate{
		TrainerId:      t.id,
		RoundNumber:    int32(round),
		UpdatedWeights: encodeWeights(t.lastWeights),
		DatasetSize:    int32(t.local.Len()),
		LocalLoss:      t.lastLoss,
	}
}
