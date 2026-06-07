package framework

import (
	"fmt"
	"strings"
)

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

type remoteRef struct {
	system  *ActorSystem
	address string
}

func (r *remoteRef) Address() string { return r.address }

func (r *remoteRef) Tell(msg Message, sender ActorRef) {
	if r.system.remote == nil {
		r.system.logger.Warn("no remote dispatcher; dropping message", "target", r.address)
		return
	}
	if err := r.system.remote.Dispatch(r.address, msg, sender); err != nil {
		r.system.logger.Warn("remote dispatch failed", "target", r.address, "error", err.Error())
	}
}

func NewLocalAddress(name string) string {
	return fmt.Sprintf("actor://local/%s", name)
}

func NewRemoteAddress(hostport, name string) string {
	return fmt.Sprintf("actor://%s/%s", hostport, name)
}

func parseAddress(address string) (authority, name string, ok bool) {
	const prefix = "actor://"
	if !strings.HasPrefix(address, prefix) {
		return "", "", false
	}
	rest := address[len(prefix):]
	i := strings.Index(rest, "/")
	if i < 0 {
		return "", "", false
	}
	return rest[:i], rest[i+1:], true
}
