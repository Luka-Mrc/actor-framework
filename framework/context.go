package framework

type ActorContext struct {
	cell   *actorCell
	sender ActorRef
}

func (c *ActorContext) Self() ActorRef { return c.cell.ref }

func (c *ActorContext) Sender() ActorRef { return c.sender }

func (c *ActorContext) System() *ActorSystem { return c.cell.system }

func (c *ActorContext) Tell(target ActorRef, msg Message) {
	if target == nil {
		return
	}
	target.Tell(msg, c.cell.ref)
}

func (c *ActorContext) Spawn(name string, props Props) ActorRef {
	return c.cell.system.spawnCell(name, props, c.cell).ref
}

func (c *ActorContext) Parent() ActorRef {
	if c.cell.parent == nil {
		return nil
	}
	return c.cell.parent.ref
}

func (c *ActorContext) Stop() {
	c.cell.stop()
}

func (c *ActorContext) Become(behavior Behavior) {
	c.cell.pendingBehavior = behavior
	c.cell.hasPendingBehavior = true
}
