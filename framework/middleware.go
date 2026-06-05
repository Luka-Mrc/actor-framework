package framework

type Handler func(ctx *ActorContext, msg Message)

type Middleware func(next Handler) Handler

func Chain(base Handler, mws ...Middleware) Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		base = mws[i](base)
	}
	return base
}
