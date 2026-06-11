package federated

import (
	"time"

	"github.com/lukam/actor-framework/federated/data"
	"github.com/lukam/actor-framework/federated/model"
	pb "github.com/lukam/actor-framework/federated/protogen"
	"github.com/lukam/actor-framework/framework"
)

type Coordinator struct {
	expectedTrainers int
	totalRounds      int
	global           *model.Weights
	testData         *data.Dataset
	done             chan float64

	agg    framework.ActorRef
	eval   framework.ActorRef
	logger framework.ActorRef

	trainers map[string]framework.ActorRef
	order    []string
	round    int
	start    time.Time
}

func NewCoordinatorProps(expectedTrainers, totalRounds int, initial *model.Weights, test *data.Dataset, done chan float64) framework.Props {
	return framework.Props{
		Strategy: framework.DefaultStrategy(),
		Factory: func() framework.Actor {
			return &Coordinator{
				expectedTrainers: expectedTrainers,
				totalRounds:      totalRounds,
				global:           initial,
				testData:         test,
				done:             done,
				trainers:         make(map[string]framework.ActorRef),
				round:            0,
			}
		},
	}
}

func (c *Coordinator) PreStart(ctx *framework.ActorContext) {
	c.start = time.Now()
	c.agg = ctx.Spawn("aggregator", NewAggregatorProps(c.expectedTrainers))
	c.eval = ctx.Spawn("evaluator", NewEvaluatorProps(c.testData))
	c.logger = ctx.Spawn("logger", NewLoggerProps())
}

func (c *Coordinator) Receive(ctx *framework.ActorContext, msg framework.Message) {
	switch m := msg.(type) {
	case *pb.RegisterTrainer:
		if _, ok := c.trainers[m.GetTrainerId()]; !ok {
			c.trainers[m.GetTrainerId()] = ctx.System().Resolve(m.GetAddress())
			c.order = append(c.order, m.GetTrainerId())
		}
		if len(c.trainers) == c.expectedTrainers && c.round == 0 {
			c.round = 1
			c.startRound(ctx)
		}

	case *pb.AggregationComplete:
		if w, err := decodeWeights(m.GetNewGlobalWeights()); err == nil {
			c.global = w
		}
		ctx.Tell(c.eval, &pb.EvaluateModel{
			RoundNumber: int32(c.round),
			Weights:     encodeWeights(c.global),
		})

	case *pb.EvaluationResult:
		ctx.System().Logger().Info("round done",
			"round", m.GetRoundNumber(), "accuracy", m.GetAccuracy(), "macro_f1", m.GetMacroF1())
		ctx.Tell(c.logger, &pb.LogEntry{
			TimestampUnixMs: time.Now().UnixMilli(),
			SourceActor:     "coordinator",
			Level:           "INFO",
			Message:         "runda završena",
		})
		if c.round < c.totalRounds {
			c.round++
			c.startRound(ctx)
		} else {
			ctx.Tell(c.logger, &pb.TrainingComplete{
				TotalRounds:   int32(c.totalRounds),
				FinalAccuracy: m.GetAccuracy(),
				DurationMs:    time.Since(c.start).Milliseconds(),
			})
			c.done <- m.GetAccuracy()
		}
	}
}

func (c *Coordinator) startRound(ctx *framework.ActorContext) {
	blob := encodeWeights(c.global)
	aggAddr := ctx.System().Advertise(c.agg.Address())
	for _, id := range c.order {
		ctx.Tell(c.trainers[id], &pb.StartRound{
			RoundNumber:       int32(c.round),
			GlobalWeights:     blob,
			AggregatorAddress: aggAddr,
		})
	}
}
