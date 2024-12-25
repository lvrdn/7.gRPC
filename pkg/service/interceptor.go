package server

import (
	gen "data/pkg/generated"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/net/context"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	status "google.golang.org/grpc/status"
)

type Interceptor struct {
	KeyMD   string
	ACLData map[string][]string
	Adm     *Admin
}

func NewInterceptor(adm *Admin, data, keyName string) (*Interceptor, error) {
	ACLdataMap := make(map[string][]string)

	err := json.Unmarshal([]byte(data), &ACLdataMap)

	if err != nil {
		return nil, fmt.Errorf("error with unmarshal acl data: %v", err)
	}

	return &Interceptor{KeyMD: keyName, ACLData: ACLdataMap, Adm: adm}, nil
}

func (ac *Interceptor) DataCollectUnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no required metadata with consumer info")
	}
	adrOfReq, ok := peer.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no required peer information")
	}

	consumer := md[ac.KeyMD][0]
	method := info.FullMethod
	host := adrOfReq.Addr.String()

	ac.Adm.Events.Mu.Lock()
	if len(ac.Adm.Events.Chans) != 0 {
		newEvent := &gen.Event{
			Consumer: consumer,
			Method:   method,
			Host:     host,
		}
		for ch := range ac.Adm.Events.Chans {
			ch <- newEvent
		}
	}
	ac.Adm.Events.Mu.Unlock()

	ac.Adm.Stats.Mu.Lock()
	if len(ac.Adm.Stats.Chans) != 0 {
		newStat := fmt.Sprintf("%s,%s", method, consumer)
		for ch := range ac.Adm.Stats.Chans {
			ch <- newStat
		}
	}
	ac.Adm.Stats.Mu.Unlock()
	reply, err := handler(ctx, req)
	return reply, err
}

func (ac *Interceptor) AccessCheckerUnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)

	if ok && len(md[ac.KeyMD]) != 0 {
		for consumer, methods := range ac.ACLData {
			if md[ac.KeyMD][0] == consumer {
				for _, method := range methods {
					if info.FullMethod == method || strings.HasSuffix(method, "/*") {
						reply, err := handler(ctx, req)
						return reply, err
					}
				}
			}
		}
	}

	return nil, status.Errorf(16, "Unauthenticated")
}

func (ac *Interceptor) DataCollectStreamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	md, ok := metadata.FromIncomingContext(ss.Context())
	if !ok {
		return fmt.Errorf("no required metadata with consumer info")
	}
	adrOfReq, ok := peer.FromContext(ss.Context())
	if !ok {
		return fmt.Errorf("no required peer information")
	}

	consumer := md[ac.KeyMD][0]
	method := info.FullMethod
	host := adrOfReq.Addr.String()

	ac.Adm.Events.Mu.Lock()
	if len(ac.Adm.Events.Chans) != 0 {
		newEvent := &gen.Event{
			Consumer: consumer,
			Method:   method,
			Host:     host,
		}
		for ch := range ac.Adm.Events.Chans {
			ch <- newEvent
		}
	}
	ac.Adm.Events.Mu.Unlock()

	if info.FullMethod == "/main.Admin/Logging" {
		ac.Adm.Events.Mu.Lock()
		ac.Adm.Events.Chans[make(chan *gen.Event)] = false
		ac.Adm.Events.Mu.Unlock()
	}

	ac.Adm.Stats.Mu.Lock()
	if len(ac.Adm.Stats.Chans) != 0 {
		newStat := fmt.Sprintf("%s,%s", method, consumer)
		for ch := range ac.Adm.Stats.Chans {
			ch <- newStat
		}
	}
	ac.Adm.Stats.Mu.Unlock()

	if info.FullMethod == "/main.Admin/Statistics" {
		ac.Adm.Stats.Mu.Lock()
		ac.Adm.Stats.Chans[make(chan string)] = false
		ac.Adm.Stats.Mu.Unlock()
	}

	reply := handler(srv, ss)
	return reply
}

func (ac *Interceptor) AccessCheckerStreamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	md, ok := metadata.FromIncomingContext(ss.Context())

	if ok && len(md[ac.KeyMD]) != 0 {
		for consumer, methods := range ac.ACLData {
			if md[ac.KeyMD][0] == consumer {
				for _, method := range methods {
					if info.FullMethod == method || strings.HasSuffix(method, "/*") {
						reply := handler(srv, ss)
						return reply
					}
				}
			}
		}
	}
	return status.Errorf(16, "Unauthenticated")
}
