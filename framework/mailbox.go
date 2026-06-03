package framework

import "sync/atomic"

const defaultMailboxCapacity = 256

type envelope struct {
	msg    Message
	sender ActorRef
}

type Mailbox struct {
	ch       chan envelope
	closed   atomic.Bool
	capacity int
}

func newMailbox(capacity int) *Mailbox {
	if capacity <= 0 {
		capacity = defaultMailboxCapacity
	}
	return &Mailbox{
		ch:       make(chan envelope, capacity),
		capacity: capacity,
	}
}

func (m *Mailbox) post(env envelope) bool {
	if m.closed.Load() {
		return false
	}
	defer func() {
		_ = recover()
	}()
	m.ch <- env
	return true
}

func (m *Mailbox) recv() (envelope, bool) {
	env, ok := <-m.ch
	return env, ok
}

func (m *Mailbox) close() {
	if m.closed.Swap(true) {
		return
	}
	close(m.ch)
}

func (m *Mailbox) Capacity() int { return m.capacity }

func (m *Mailbox) Len() int { return len(m.ch) }
