package framework

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
)

type ActorSystem struct {
	name   string
	logger *slog.Logger

	mu     sync.Mutex
	actors map[string]*actorCell // ime -> cell, za lokalno pronalaženje

	closed atomic.Bool
}

type Option func(*ActorSystem)

func WithLogger(l *slog.Logger) Option {
	return func(s *ActorSystem) { s.logger = l }
}

func NewActorSystem(name string, opts ...Option) *ActorSystem {
	s := &ActorSystem{
		name:   name,
		logger: slog.Default(),
		actors: make(map[string]*actorCell),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *ActorSystem) Name() string { return s.name }

func (s *ActorSystem) Logger() *slog.Logger { return s.logger }

func (s *ActorSystem) Spawn(name string, props Props) (ActorRef, error) {
	if props.Factory == nil {
		return nil, fmt.Errorf("framework: Props.Factory je obavezan")
	}
	return s.spawnCell(name, props).ref, nil
}

func (s *ActorSystem) MustSpawn(name string, props Props) ActorRef {
	ref, err := s.Spawn(name, props)
	if err != nil {
		panic(err)
	}
	return ref
}

func (s *ActorSystem) Shutdown(_ context.Context) {
	if !s.closed.CompareAndSwap(false, true) {
		return
	}
	s.mu.Lock()
	cells := make([]*actorCell, 0, len(s.actors))
	for _, c := range s.actors {
		cells = append(cells, c)
	}
	s.mu.Unlock()
	for _, c := range cells {
		c.stop()
	}
	for _, c := range cells {
		<-c.done
	}
}

func (s *ActorSystem) spawnCell(name string, props Props) *actorCell {
	cell := newActorCell(s, name, props)
	s.mu.Lock()
	s.actors[name] = cell
	s.mu.Unlock()
	go cell.run()
	return cell
}

func (s *ActorSystem) unregisterCell(cell *actorCell) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cur, ok := s.actors[cell.name]; ok && cur == cell {
		delete(s.actors, cell.name)
	}
}

type actorCell struct {
	system  *ActorSystem
	name    string
	ref     *localRef
	props   Props
	mailbox *Mailbox

	actor    Actor
	behavior Behavior

	pendingBehavior    Behavior
	hasPendingBehavior bool

	stopOnce sync.Once
	done     chan struct{}
}

func newActorCell(sys *ActorSystem, name string, props Props) *actorCell {
	cell := &actorCell{
		system:  sys,
		name:    name,
		props:   props,
		mailbox: newMailbox(props.MailboxCapacity),
		done:    make(chan struct{}),
	}
	cell.ref = &localRef{address: NewLocalAddress(name), cell: cell}
	return cell
}

func (c *actorCell) deliver(env envelope) {
	if !c.mailbox.post(env) {
		c.system.logger.Warn("dead letter",
			"actor", c.name, "msg_type", fmt.Sprintf("%T", env.msg))
	}
}

func (c *actorCell) run() {
	defer close(c.done)
	c.actor = c.props.Factory()
	c.behavior = c.actor.Receive
	c.runPreStart()

	for {
		env, ok := c.mailbox.recv()
		if !ok {
			break
		}
		c.dispatchOne(env)
	}

	c.runPostStop()
	c.system.unregisterCell(c)
}

func (c *actorCell) dispatchOne(env envelope) {
	ctx := &ActorContext{cell: c, sender: env.sender}
	c.behavior(ctx, env.msg)
	if c.hasPendingBehavior {
		c.behavior = c.pendingBehavior
		c.pendingBehavior = nil
		c.hasPendingBehavior = false
	}
}

func (c *actorCell) runPreStart() {
	ctx := &ActorContext{cell: c}
	invokePreStart(c.actor, ctx)
}

func (c *actorCell) runPostStop() {
	ctx := &ActorContext{cell: c}
	invokePostStop(c.actor, ctx)
}

func (c *actorCell) stop() {
	c.stopOnce.Do(func() {
		c.mailbox.close()
	})
}
