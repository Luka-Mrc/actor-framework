package framework

type Message = any

type Actor interface {
	Receive(ctx *ActorContext, msg Message)
}

type Behavior func(ctx *ActorContext, msg Message)

type ActorFunc Behavior

func (f ActorFunc) Receive(ctx *ActorContext, msg Message) { f(ctx, msg) }

type PreStarter interface {
	PreStart(ctx *ActorContext)
}

type PostStopper interface {
	PostStop(ctx *ActorContext)
}

type Props struct {
	Factory func() Actor

	MailboxCapacity int

	Strategy SupervisorStrategy

	Middleware []Middleware
}
