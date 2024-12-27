package main

import (
	service "app/pkg/service"
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

const (
	// какой адрес-порт слушать серверу
	listenAddr string = "127.0.0.1:8082"

	// кого по каким методам пускать
	ACLData string = `{
	"logger1":          ["/main.Admin/Logging"],
	"logger2":          ["/main.Admin/Logging"],
	"stat1":            ["/main.Admin/Statistics"],
	"stat2":            ["/main.Admin/Statistics"],
	"biz_user":         ["/main.Biz/Check", "/main.Biz/Add"],
	"biz_admin":        ["/main.Biz/*"],
	"after_disconnect": ["/main.Biz/Add"]
}`
)

func main() {
	ctx, finish := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)
	err := service.StartMyMicroservice(ctx, listenAddr, ACLData, wg)
	if err != nil {
		log.Fatalf("start app eror: [%s]\n", err.Error())
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	finish()
	wg.Wait()
}
