package federated

import (
	"sort"
	"time"

	"github.com/lukam/actor-framework/federated/data"
	"github.com/lukam/actor-framework/federated/model"
	pb "github.com/lukam/actor-framework/federated/protogen"
	"github.com/lukam/actor-framework/framework"
)

const maxSyncRetries = 60

type retryTick struct {
	round   int
	attempt int
}

type peerUpdate struct {
	weights *model.Weights
	size    int
}

type PeerCoordinator struct {
	id          string
	peerAddrs   []string
	numPeers    int
	local       *data.Dataset
	test        *data.Dataset
	epochs      int
	lr          float64
	totalRounds int
	done        chan float64

	global   *model.Weights
	round    int
	buffer   map[int]map[string]peerUpdate
	start    time.Time
	finished bool
}

func NewPeerCoordinatorProps(id string, peerAddrs []string, local, test *data.Dataset, epochs int, lr float64, totalRounds int, initial *model.Weights, done chan float64) framework.Props {
	return framework.Props{Factory: func() framework.Actor {
		return &PeerCoordinator{
			id:          id,
			peerAddrs:   peerAddrs,
			numPeers:    len(peerAddrs) + 1,
			local:       local,
			test:        test,
			epochs:      epochs,
			lr:          lr,
			totalRounds: totalRounds,
			done:        done,
			global:      initial,
			round:       1,
			buffer:      make(map[int]map[string]peerUpdate),
		}
	}}
}

func (c *PeerCoordinator) PreStart(ctx *framework.ActorContext) {
	c.start = time.Now()
	c.trainAndBroadcast(ctx)
	c.advance(ctx)
}

func (c *PeerCoordinator) Receive(ctx *framework.ActorContext, msg framework.Message) {
	switch m := msg.(type) {
	case *pb.PeerSync:
		if c.finished {
			return
		}
		r := int(m.GetRoundNumber())
		if r < c.round {
			return
		}
		c.ensureRound(r)
		if _, dup := c.buffer[r][m.GetNodeId()]; dup {
			return
		}
		w, err := decodeWeights(m.GetWeights())
		if err != nil {
			ctx.System().Logger().Error("peer: decode weights", "node", c.id, "from", m.GetNodeId(), "error", err.Error())
			return
		}
		c.buffer[r][m.GetNodeId()] = peerUpdate{weights: w, size: int(m.GetDatasetSize())}
		c.advance(ctx)

	case retryTick:
		if c.finished || m.round != c.round {
			return
		}
		if len(c.buffer[c.round]) >= c.numPeers {
			return
		}
		if own, ok := c.buffer[c.round][c.id]; ok {
			c.broadcast(ctx, c.round, own.weights)
		}
		if m.attempt < maxSyncRetries {
			c.scheduleRetry(ctx, c.round, m.attempt+1)
		}
	}
}

func (c *PeerCoordinator) advance(ctx *framework.ActorContext) {
	for !c.finished {
		buf := c.buffer[c.round]
		if len(buf) < c.numPeers {
			return
		}
		c.global = aggregatePeers(buf)
		m := model.Evaluate(c.global, c.test.X, c.test.Y)
		ctx.System().Logger().Info("peer round done",
			"node", c.id, "round", c.round, "accuracy", m.Accuracy, "macro_f1", m.MacroF1)
		delete(c.buffer, c.round)

		if c.round >= c.totalRounds {
			c.finish(ctx, m.Accuracy)
			return
		}
		c.round++
		c.trainAndBroadcast(ctx)
	}
}

func (c *PeerCoordinator) trainAndBroadcast(ctx *framework.ActorContext) {
	localW := cloneWeights(c.global)
	for e := 0; e < c.epochs; e++ {
		model.TrainEpoch(localW, c.local.X, c.local.Y, c.lr)
	}
	c.ensureRound(c.round)
	c.buffer[c.round][c.id] = peerUpdate{weights: localW, size: c.local.Len()}
	c.broadcast(ctx, c.round, localW)
	c.scheduleRetry(ctx, c.round, 1)
}

func (c *PeerCoordinator) broadcast(ctx *framework.ActorContext, round int, w *model.Weights) {
	msg := &pb.PeerSync{
		NodeId:      c.id,
		RoundNumber: int32(round),
		Weights:     encodeWeights(w),
		DatasetSize: int32(c.local.Len()),
	}
	for _, addr := range c.peerAddrs {
		ctx.Tell(ctx.System().Resolve(addr), msg)
	}
}

func (c *PeerCoordinator) scheduleRetry(ctx *framework.ActorContext, round, attempt int) {
	self := ctx.Self()
	go func() {
		time.Sleep(time.Second)
		self.Tell(retryTick{round: round, attempt: attempt}, self)
	}()
}

func (c *PeerCoordinator) finish(ctx *framework.ActorContext, acc float64) {
	c.finished = true
	ctx.System().Logger().Info("peer training complete",
		"node", c.id, "rounds", c.totalRounds,
		"final_accuracy", acc, "duration_ms", time.Since(c.start).Milliseconds())
	if c.done != nil {
		c.done <- acc
	}
}

func (c *PeerCoordinator) ensureRound(r int) {
	if c.buffer[r] == nil {
		c.buffer[r] = make(map[string]peerUpdate)
	}
}

func aggregatePeers(buf map[string]peerUpdate) *model.Weights {
	ids := make([]string, 0, len(buf))
	for id := range buf {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	updates := make([]*model.Weights, len(ids))
	sizes := make([]int, len(ids))
	for i, id := range ids {
		updates[i] = buf[id].weights
		sizes[i] = buf[id].size
	}
	return fedAvg(updates, sizes)
}

func cloneWeights(w *model.Weights) *model.Weights {
	clone, _ := decodeWeights(encodeWeights(w))
	return clone
}
