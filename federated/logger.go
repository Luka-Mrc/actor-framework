package federated

import (
	pb "github.com/lukam/actor-framework/federated/protogen"
	"github.com/lukam/actor-framework/framework"
)

type Logger struct{}

func NewLoggerProps() framework.Props {
	return framework.Props{Factory: func() framework.Actor { return &Logger{} }}
}

func (l *Logger) Receive(ctx *framework.ActorContext, msg framework.Message) {
	log := ctx.System().Logger()
	switch m := msg.(type) {
	case *pb.LogEntry:
		log.Info("fl", "source", m.GetSourceActor(), "level", m.GetLevel(), "message", m.GetMessage())
	case *pb.TrainingComplete:
		log.Info("training complete",
			"rounds", m.GetTotalRounds(),
			"final_accuracy", m.GetFinalAccuracy(),
			"duration_ms", m.GetDurationMs())
	}
}
