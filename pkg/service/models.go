package server

import (
	gen "app/pkg/generated"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"
)

type Biz struct {
	// some fields
}

func (b *Biz) Check(ctx context.Context, n *gen.Nothing) (*gen.Nothing, error) {
	//log.Println("method check do some work")
	return &gen.Nothing{}, nil
}

func (b *Biz) Add(ctx context.Context, n *gen.Nothing) (*gen.Nothing, error) {
	//log.Println("method add do some work")
	return &gen.Nothing{}, nil
}

func (b *Biz) Test(ctx context.Context, n *gen.Nothing) (*gen.Nothing, error) {
	//log.Println("method test do some work")
	return &gen.Nothing{}, nil
}

type Admin struct {
	Events *EventsInfo
	Stats  *StatsInfo
}

type EventsInfo struct {
	Chans map[chan *gen.Event]bool
	Mu    *sync.Mutex
}

type StatsInfo struct {
	Chans map[chan string]bool
	Mu    *sync.Mutex
}

func (adm *Admin) Logging(n *gen.Nothing, log gen.Admin_LoggingServer) error {

	var chIn chan *gen.Event
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

func (adm *Admin) Statistics(s *gen.StatInterval, stat gen.Admin_StatisticsServer) error {

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

	sendStat := &gen.Stat{
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
			sendStat = &gen.Stat{
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
