package server

import (
	gen "data/pkg/generated"
	"net"
	"sync"

	"golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

func StartMyMicroservice(ctx context.Context, listenAddr string, data string) error {

	biz := &Biz{}

	admin := &Admin{
		Events: &EventsInfo{
			Chans: make(map[chan *gen.Event]bool),
			Mu:    &sync.Mutex{},
		},
		Stats: &StatsInfo{
			Chans: make(map[chan string]bool),
			Mu:    &sync.Mutex{},
		},
	}

	interceptor, err := NewInterceptor(admin, data, "consumer")
	if err != nil {
		return err
	}

	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptor.AccessCheckerUnaryInterceptor,
			interceptor.DataCollectUnaryInterceptor,
		),
		grpc.ChainStreamInterceptor(
			interceptor.AccessCheckerStreamInterceptor,
			interceptor.DataCollectStreamInterceptor,
		),
	)

	gen.RegisterAdminServer(server, admin)
	gen.RegisterBizServer(server, biz)

	lis, err := net.Listen("tcp", listenAddr)

	if err != nil {
		return err
	}

	go func() {

		<-ctx.Done()
		server.GracefulStop()
	}()

	go server.Serve(lis)

	return nil
}
