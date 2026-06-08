package remote

import (
	"fmt"
	"sync"

	"google.golang.org/protobuf/proto"
)

var (
	codecMu  sync.RWMutex
	registry = make(map[string]func() proto.Message)
)

func Register(msg proto.Message) {
	name := typeName(msg)
	codecMu.Lock()
	defer codecMu.Unlock()
	registry[name] = func() proto.Message {
		return msg.ProtoReflect().New().Interface()
	}
}

func typeName(msg proto.Message) string {
	return string(msg.ProtoReflect().Descriptor().FullName())
}

func Encode(msg proto.Message) (string, []byte, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return "", nil, err
	}
	return typeName(msg), data, nil
}

func Decode(name string, data []byte) (proto.Message, error) {
	codecMu.RLock()
	factory, ok := registry[name]
	codecMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("remote: nepoznat tip poruke %q (nije registrovan)", name)
	}
	msg := factory()
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return msg, nil
}
