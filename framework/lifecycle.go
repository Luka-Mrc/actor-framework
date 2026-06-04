package framework

func invokePreStart(actor Actor, ctx *ActorContext) {
	if ps, ok := actor.(PreStarter); ok {
		ps.PreStart(ctx)
	}
}

func invokePostStop(actor Actor, ctx *ActorContext) {
	if ps, ok := actor.(PostStopper); ok {
		defer func() { _ = recover() }()
		ps.PostStop(ctx)
	}
}
