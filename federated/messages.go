package federated

import (
	pb "github.com/lukam/actor-framework/federated/protogen"
	"github.com/lukam/actor-framework/framework/remote"
)

func init() { RegisterMessages() }

func RegisterMessages() {
	remote.Register(&pb.RegisterTrainer{})
	remote.Register(&pb.StartRound{})
	remote.Register(&pb.LocalUpdate{})
	remote.Register(&pb.AggregationComplete{})
	remote.Register(&pb.EvaluateModel{})
	remote.Register(&pb.EvaluationResult{})
	remote.Register(&pb.TrainingComplete{})
	remote.Register(&pb.LogEntry{})
	remote.Register(&pb.PeerSync{})
}
