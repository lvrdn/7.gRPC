package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"
	grpc "google.golang.org/grpc"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	status "google.golang.org/grpc/status"
)

type Biz struct {
	// some fields
}

func (b *Biz) Check(ctx context.Context, n *Nothing) (*Nothing, error) {
	//fmt.Println("method check do some work")
	return &Nothing{}, nil
}

func (b *Biz) Add(ctx context.Context, n *Nothing) (*Nothing, error) {
	//fmt.Println("method add do some work")
	return &Nothing{}, nil
}

func (b *Biz) Test(ctx context.Context, n *Nothing) (*Nothing, error) {
	//fmt.Println("method test do some work")
	return &Nothing{}, nil
}

func (b *Biz) mustEmbedUnimplementedBizServer() {}

type Admin struct {
	Events *EventsInfo
	Stats  *StatsInfo
}

type EventsInfo struct {
	Chans map[chan *Event]bool
	Mu    *sync.Mutex
}

type StatsInfo struct {
	Chans map[chan string]bool
	Mu    *sync.Mutex
}

func (adm *Admin) Logging(n *Nothing, log Admin_LoggingServer) error {

	var chIn chan *Event
	adm.Events.Mu.Lock()
	for ch, busy := range adm.Events.Chans {
		if busy {
			continue
		}
		chIn = ch
		adm.Events.Chans[ch] = true
		break
	}
	adm.Events.Mu.Unlock()

LOOP:
	for {
		select {
		case <-log.Context().Done():
			break LOOP
		case newEvent := <-chIn:
			log.Send(newEvent)
		default:
			continue LOOP
		}
	}

	adm.Events.Mu.Lock()
	delete(adm.Events.Chans, chIn)
	adm.Events.Mu.Unlock()

	return nil
}

func (adm *Admin) Statistics(s *StatInterval, stat Admin_StatisticsServer) error {

	var chIn chan string
	adm.Stats.Mu.Lock()
	for ch, busy := range adm.Stats.Chans {
		if busy {
			continue
		}
		chIn = ch
		adm.Stats.Chans[ch] = true
		break
	}
	adm.Stats.Mu.Unlock()

	sendStat := &Stat{
		ByMethod:   make(map[string]uint64),
		ByConsumer: make(map[string]uint64),
	}
	mu := &sync.Mutex{}
	start := time.Now()

LOOP:
	for {

		if time.Since(start) > time.Duration(s.IntervalSeconds)*time.Second {
			start = time.Now()

			mu.Lock()
			stat.Send(sendStat)
			sendStat = &Stat{
				ByMethod:   make(map[string]uint64),
				ByConsumer: make(map[string]uint64),
			}
			mu.Unlock()
		}

		select {
		case <-stat.Context().Done():
			break LOOP
		case statData := <-chIn:
			s := strings.Split(statData, ",")

			mu.Lock()
			sendStat.ByMethod[s[0]]++
			sendStat.ByConsumer[s[1]]++
			mu.Unlock()

		default:
			continue LOOP
		}
	}

	adm.Stats.Mu.Lock()
	delete(adm.Stats.Chans, chIn)
	adm.Stats.Mu.Unlock()

	return nil
}

func (adm *Admin) mustEmbedUnimplementedAdminServer() {}

func StartMyMicroservice(ctx context.Context, listenAddr string, data string) error {

	biz := &Biz{}
	admin := &Admin{
		Events: &EventsInfo{
			Chans: make(map[chan *Event]bool),
			Mu:    &sync.Mutex{},
		},
		Stats: &StatsInfo{
			Chans: make(map[chan string]bool),
			Mu:    &sync.Mutex{},
		},
	}

	checker, err := NewAccessChecker(admin, data, "consumer")
	if err != nil {
		return err
	}

	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			checker.AccessCheckerUnaryInterceptor,
			checker.DataCollectUnaryInterceptor,
		),
		grpc.ChainStreamInterceptor(
			checker.AccessCheckerStreamInterceptor,
			checker.DataCollectStreamInterceptor,
		),
	)

	RegisterAdminServer(server, admin)
	RegisterBizServer(server, biz)

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

type AccessChecker struct {
	KeyMD   string
	ACLData map[string][]string
	Adm     *Admin
}

func NewAccessChecker(adm *Admin, data, keyName string) (*AccessChecker, error) {
	ACLdataMap := make(map[string][]string)

	err := json.Unmarshal([]byte(data), &ACLdataMap)

	if err != nil {
		return nil, fmt.Errorf("error with unmarshal acl data: %v", err)
	}

	return &AccessChecker{KeyMD: keyName, ACLData: ACLdataMap, Adm: adm}, nil
}

func (ac *AccessChecker) DataCollectUnaryInterceptor(
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
		newEvent := &Event{
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

func (ac *AccessChecker) AccessCheckerUnaryInterceptor(
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

func (ac *AccessChecker) DataCollectStreamInterceptor(
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
		newEvent := &Event{
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
		ac.Adm.Events.Chans[make(chan *Event)] = false
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

func (ac *AccessChecker) AccessCheckerStreamInterceptor(
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
