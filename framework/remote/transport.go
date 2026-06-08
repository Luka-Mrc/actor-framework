package remote

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"github.com/lukam/actor-framework/framework"
	remotepb "github.com/lukam/actor-framework/framework/remote/proto"
)

type RemoteSystem struct {
	remotepb.UnimplementedActorTransportServer

	system *framework.ActorSystem

	listen     string
	advertised string

	grpcServer *grpc.Server

	mu    sync.Mutex
	conns map[string]*grpc.ClientConn
}

func NewRemoteSystem(system *framework.ActorSystem, listen string) *RemoteSystem {
	rs := &RemoteSystem{
		system:     system,
		listen:     listen,
		advertised: listen,
		conns:      make(map[string]*grpc.ClientConn),
	}
	system.SetRemoteDispatcher(rs)
	return rs
}

func (rs *RemoteSystem) SetAdvertisedAddress(addr string) { rs.advertised = addr }

func (rs *RemoteSystem) ServeAsync() error {
	lis, err := net.Listen("tcp", rs.listen)
	if err != nil {
		return err
	}
	rs.grpcServer = grpc.NewServer()
	remotepb.RegisterActorTransportServer(rs.grpcServer, rs)
	go func() { _ = rs.grpcServer.Serve(lis) }()
	return nil
}

func (rs *RemoteSystem) Serve() error {
	lis, err := net.Listen("tcp", rs.listen)
	if err != nil {
		return err
	}
	rs.grpcServer = grpc.NewServer()
	remotepb.RegisterActorTransportServer(rs.grpcServer, rs)
	return rs.grpcServer.Serve(lis)
}

func (rs *RemoteSystem) Stop() {
	if rs.grpcServer != nil {
		rs.grpcServer.GracefulStop()
	}
	rs.mu.Lock()
	for _, c := range rs.conns {
		_ = c.Close()
	}
	rs.conns = make(map[string]*grpc.ClientConn)
	rs.mu.Unlock()
}

func (rs *RemoteSystem) Deliver(_ context.Context, env *remotepb.Envelope) (*remotepb.DeliverAck, error) {
	msg, err := Decode(env.GetTypeName(), env.GetPayload())
	if err != nil {
		return nil, err
	}
	var sender framework.ActorRef
	if addr := env.GetSender(); addr != "" {
		sender = rs.system.Resolve(addr)
	}
	if err := rs.system.DeliverLocal(env.GetTargetActor(), msg, sender); err != nil {
		return nil, err
	}
	return &remotepb.DeliverAck{}, nil
}

func (rs *RemoteSystem) Dispatch(targetAddress string, msg framework.Message, sender framework.ActorRef) error {
	hostport, name, ok := parseHostPortName(targetAddress)
	if !ok {
		return fmt.Errorf("remote: neispravna adresa %q", targetAddress)
	}
	pm, ok := msg.(proto.Message)
	if !ok {
		return fmt.Errorf("remote: poruka %T nije proto.Message (mora biti za udaljeno slanje)", msg)
	}
	typeName, payload, err := Encode(pm)
	if err != nil {
		return err
	}

	client, err := rs.client(hostport)
	if err != nil {
		return err
	}
	senderAddr := ""
	if sender != nil {
		senderAddr = rs.advertise(sender.Address())
	}
	_, err = client.Deliver(context.Background(), &remotepb.Envelope{
		TargetActor: name,
		TypeName:    typeName,
		Payload:     payload,
		Sender:      senderAddr,
	})
	return err
}

func (rs *RemoteSystem) client(hostport string) (remotepb.ActorTransportClient, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	conn, ok := rs.conns[hostport]
	if !ok {
		var err error
		conn, err = grpc.NewClient(hostport, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
		}
		rs.conns[hostport] = conn
	}
	return remotepb.NewActorTransportClient(conn), nil
}

func (rs *RemoteSystem) advertise(addr string) string {
	if hostport, name, ok := parseHostPortName(addr); ok && hostport == "local" {
		return framework.NewRemoteAddress(rs.advertised, name)
	}
	return addr
}

func parseHostPortName(addr string) (hostport, name string, ok bool) {
	const prefix = "actor://"
	if !strings.HasPrefix(addr, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(addr, prefix)
	i := strings.Index(rest, "/")
	if i < 0 {
		return "", "", false
	}
	return rest[:i], rest[i+1:], true
}
