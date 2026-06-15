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
		RoundNumber:     m.GetRoundNumber(),
		Accuracy:        metrics.Accuracy,
		ClassMetrics:    classMetrics(metrics),
		ConfusionMatrix: flattenConfusion(metrics.Confusion),
	})
}

func classMetrics(m model.Metrics) map[string]*pb.F1Score {
	out := make(map[string]*pb.F1Score, len(m.F1))
	for c := range m.F1 {
		out[data.Class(c).String()] = &pb.F1Score{
			Precision: m.Precision[c],
			Recall:    m.Recall[c],
			F1:        m.F1[c],
		}
	}
	return out
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
