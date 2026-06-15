package federated

import (
	"time"

	"github.com/lukam/actor-framework/crdt"
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

	trainers        map[string]framework.ActorRef
	order           []string
	participants    *crdt.ORSet
	round           int
	aggregatedRound int
	roundTimeout    time.Duration
	finished        bool
	start           time.Time
}

type roundTimeoutMsg struct{ round int }

func NewCoordinatorProps(expectedTrainers, totalRounds int, initial *model.Weights, test *data.Dataset, roundTimeout time.Duration, done chan float64) framework.Props {
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
				participants:     crdt.NewORSet("coordinator"),
				round:            0,
				roundTimeout:     roundTimeout,
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
			c.participants.Add(m.GetTrainerId())
		}
		if len(c.trainers) == c.expectedTrainers && c.round == 0 {
			ctx.System().Logger().Info("all trainers registered", "participants", c.participants.Elements())
			c.round = 1
			c.startRound(ctx)
		}

	case *pb.AggregationComplete:
		r := int(m.GetRoundNumber())
		if r != c.round || c.aggregatedRound >= c.round {
			return
		}
		c.aggregatedRound = c.round
		if w, err := decodeWeights(m.GetNewGlobalWeights()); err == nil {
			c.global = w
		}
		ctx.Tell(c.eval, &pb.EvaluateModel{
			RoundNumber: int32(c.round),
			Weights:     encodeWeights(c.global),
		})

	case roundTimeoutMsg:

		if m.round == c.round && c.aggregatedRound < c.round {
			ctx.System().Logger().Warn("round timeout; finalizing with received updates", "round", c.round)
			ctx.Tell(c.agg, finalizeRound{round: c.round})
			c.scheduleTimeout(ctx, c.round)
		}

	case framework.SupervisionAlert:
		c.handleSupervision(ctx, m)

	case *pb.EvaluationResult:
		ctx.System().Logger().Info("round done",
			"round", m.GetRoundNumber(), "accuracy", m.GetAccuracy(), "macro_f1", macroF1(m.GetClassMetrics()))
		ctx.Tell(c.logger, &pb.LogEntry{
			TimestampUnixMs: time.Now().UnixMilli(),
			SourceActor:     "coordinator",
			Level:           "INFO",
			Message:         "runda završena",
		})
		if c.round < c.totalRounds {
			c.round++
			c.startRound(ctx)
		} else if !c.finished {
			c.finished = true
			ctx.Tell(c.logger, &pb.TrainingComplete{
				TotalRounds:   int32(c.totalRounds),
				FinalAccuracy: m.GetAccuracy(),
				Duration:      time.Since(c.start).Milliseconds(),
			})
			if c.done != nil {
				c.done <- m.GetAccuracy()
			}
		}
	}
}

func (c *Coordinator) handleSupervision(ctx *framework.ActorContext, m framework.SupervisionAlert) {
	addr := ""
	if m.Child != nil {
		addr = m.Child.Address()
	}
	switch {
	case c.agg != nil && addr == c.agg.Address():

		ctx.System().Logger().Warn("aggregator restarted; resending round", "round", c.round)
		if c.round > 0 && c.aggregatedRound < c.round {
			c.startRound(ctx)
		}
	case c.eval != nil && addr == c.eval.Address():
		ctx.System().Logger().Warn("evaluator restarted", "round", c.round)
	default:
		ctx.System().Logger().Warn("child restarted", "child", addr)
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
	c.scheduleTimeout(ctx, c.round)
}

func (c *Coordinator) scheduleTimeout(ctx *framework.ActorContext, round int) {
	if c.roundTimeout <= 0 {
		return
	}
	self := ctx.Self()
	d := c.roundTimeout
	go func() {
		time.Sleep(d)
		self.Tell(roundTimeoutMsg{round: round}, self)
	}()
}

func macroF1(cm map[string]*pb.F1Score) float64 {
	if len(cm) == 0 {
		return 0
	}
	var sum float64
	for _, s := range cm {
		sum += s.GetF1()
	}
	return sum / float64(len(cm))
}
