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
	actors map[string]*actorCell

	guardian SupervisorStrategy

	globalMiddleware []Middleware

	closed atomic.Bool
}

type Option func(*ActorSystem)

func WithLogger(l *slog.Logger) Option {
	return func(s *ActorSystem) { s.logger = l }
}

func WithGuardianStrategy(st SupervisorStrategy) Option {
	return func(s *ActorSystem) { s.guardian = st }
}

func WithGlobalMiddleware(mws ...Middleware) Option {
	return func(s *ActorSystem) { s.globalMiddleware = append(s.globalMiddleware, mws...) }
}

func NewActorSystem(name string, opts ...Option) *ActorSystem {
	s := &ActorSystem{
		name:     name,
		logger:   slog.Default(),
		actors:   make(map[string]*actorCell),
		guardian: DefaultStrategy(),
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
	return s.spawnCell(name, props, nil).ref, nil
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

func (s *ActorSystem) spawnCell(name string, props Props, parent *actorCell) *actorCell {
	cell := newActorCell(s, name, props, parent)
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
	parent  *actorCell

	actor    Actor
	behavior Behavior
	handler  Handler

	pendingBehavior    Behavior
	hasPendingBehavior bool

	stopOnce sync.Once
	done     chan struct{}
}

func newActorCell(sys *ActorSystem, name string, props Props, parent *actorCell) *actorCell {
	cell := &actorCell{
		system:  sys,
		name:    name,
		props:   props,
		mailbox: newMailbox(props.MailboxCapacity),
		parent:  parent,
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
	c.initActor()

	for {
		env, ok := c.mailbox.recv()
		if !ok {
			break
		}
		if !c.dispatchOne(env) {
			break
		}
	}

	c.runPostStop()
	c.system.unregisterCell(c)
}

func (c *actorCell) initActor() {
	c.actor = c.props.Factory()
	c.behavior = c.actor.Receive
	c.pendingBehavior = nil
	c.hasPendingBehavior = false
	c.rebuildHandler()
	c.runPreStart()
}

func (c *actorCell) rebuildHandler() {
	mws := make([]Middleware, 0, len(c.system.globalMiddleware)+len(c.props.Middleware))
	mws = append(mws, c.system.globalMiddleware...)
	mws = append(mws, c.props.Middleware...)
	c.handler = Chain(Handler(c.behavior), mws...)
}

func (c *actorCell) dispatchOne(env envelope) (alive bool) {
	if esc, ok := env.msg.(escalation); ok {
		return c.handlePanic(esc.reason)
	}

	alive = true
	defer func() {
		if r := recover(); r != nil {
			alive = c.handlePanic(r)
		}
	}()

	ctx := &ActorContext{cell: c, sender: env.sender}
	c.handler(ctx, env.msg)
	if c.hasPendingBehavior {
		c.behavior = c.pendingBehavior
		c.pendingBehavior = nil
		c.hasPendingBehavior = false
		c.rebuildHandler()
	}
	return true
}

func (c *actorCell) handlePanic(reason any) bool {
	d := c.supervisor().Decide(SupervisionAlert{Child: c.ref, Reason: reason})
	c.system.logger.Error("actor panic",
		"actor", c.name, "reason", fmt.Sprintf("%v", reason), "directive", d.String())

	switch d {
	case Restart:
		c.initActor()
		return true
	case Escalate:
		c.escalate(reason)
		return false
	default: // Stop
		c.stop()
		return false
	}
}

func (c *actorCell) supervisor() SupervisorStrategy {
	if c.parent != nil {
		if st := c.parent.props.Strategy; st != nil {
			return st
		}
		return DefaultStrategy()
	}
	return c.system.guardian
}

func (c *actorCell) escalate(reason any) {
	c.stop()
	if c.parent != nil {
		c.parent.deliver(envelope{msg: escalation{child: c.ref, reason: reason}})
		return
	}
	c.system.logger.Error("escalation reached top level; actor stopped", "actor", c.name)
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
