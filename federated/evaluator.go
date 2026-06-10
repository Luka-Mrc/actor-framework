package federated

import (
	"github.com/lukam/actor-framework/federated/data"
	"github.com/lukam/actor-framework/federated/model"
	pb "github.com/lukam/actor-framework/federated/protogen"
	"github.com/lukam/actor-framework/framework"
)

type Evaluator struct {
	test *data.Dataset
}

func NewEvaluatorProps(test *data.Dataset) framework.Props {
	return framework.Props{Factory: func() framework.Actor { return &Evaluator{test: test} }}
}

func (e *Evaluator) Receive(ctx *framework.ActorContext, msg framework.Message) {
	m, ok := msg.(*pb.EvaluateModel)
	if !ok {
		return
	}
	w, err := decodeWeights(m.GetWeights())
	if err != nil {
		ctx.System().Logger().Error("evaluator: decode weights", "error", err.Error())
		return
	}
	metrics := model.Evaluate(w, e.test.X, e.test.Y)

	ctx.Tell(ctx.Parent(), &pb.EvaluationResult{
		RoundNumber: m.GetRoundNumber(),
		Accuracy:    metrics.Accuracy,
		MacroF1:     metrics.MacroF1,
		ClassF1:     metrics.F1,
		Confusion:   flattenConfusion(metrics.Confusion),
	})
}

func flattenConfusion(conf [][]int) []int32 {
	out := make([]int32, 0, len(conf)*len(conf))
	for i := range conf {
		for j := range conf[i] {
			out = append(out, int32(conf[i][j]))
		}
	}
	return out
}
