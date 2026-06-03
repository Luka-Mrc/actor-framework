package framework

import "fmt"

type ActorRef interface {
	Address() string
	Tell(msg Message, sender ActorRef)
}

type localRef struct {
	address string
	cell    *actorCell
}

func (r *localRef) Address() string { return r.address }

func (r *localRef) Tell(msg Message, sender ActorRef) {
	cell := r.cell
	if cell == nil {
		return
	}
	cell.deliver(envelope{msg: msg, sender: sender})
}

func NewLocalAddress(name string) string {
	return fmt.Sprintf("actor://local/%s", name)
}
