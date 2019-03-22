package main

import (
	"context"

	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/sereiner/parrot/servers/rpc/client"
	"github.com/sereiner/parrot/servers/rpc/codec"
	"github.com/sereiner/parrot/servers/rpc/registry"
	"github.com/sereiner/parrot/servers/rpc/registry/memory"
	"github.com/sereiner/parrot/servers/rpc/server"
	"github.com/sereiner/parrot/servers/rpc/service"
)

const callTimes = 1

var s server.RPCServer

func main() {
	StartServer()
	time.Sleep(1e9)
	start := time.Now()
	for i := 0; i < callTimes; i++ {
		MakeCall(codec.GOB)
		//MakeCall(codec.MessagePack)
	}
	cost := time.Now().Sub(start)
	log.Printf("cost:%s", cost)
	StopServer()
}

func StopServer() {
	s.Close()
}

var Registry = memory.NewInMemoryRegistry()

func StartServer() {
	go func() {
		serverOpt := server.DefaultOption
		serverOpt.RegisterOption.AppKey = "my-app"
		serverOpt.Registry = Registry
		s = server.NewRPCServer(serverOpt)
		err := s.Register(service.Arith{}, make(map[string]string))
		if err != nil {
			log.Println("err!!!" + err.Error())
		}
		port := 8880
		s.Serve("tcp", ":"+strconv.Itoa(port))
	}()
}

func MakeCall(t codec.SerializeType) {
	op := &client.DefaultSGOption
	op.AppKey = "my-app"
	op.SerializeType = t
	op.RequestTimeout = time.Millisecond * 100
	op.DialTimeout = time.Millisecond * 100
	op.FailMode = client.FailRetry
	op.Retries = 3

	r := registry.NewPeer2PeerRegistry()
	r.Register(registry.RegisterOption{}, registry.Provider{ProviderKey: "tcp@:8880", Network: "tcp", Addr: ":8880"})
	op.Registry = r

	c := client.NewSGClient(*op)

	args := service.Args{A: rand.Intn(200), B: rand.Intn(100)}
	reply := &service.Reply{}
	ctx := context.Background()
	err := c.Call(ctx, "Arith.Add", args, reply)
	if err != nil {
		log.Println("err!!!" + err.Error())
	} else if reply.C != args.A+args.B {
		log.Printf("%d + %d != %d", args.A, args.B, reply.C)
	}

	args = service.Args{A: rand.Intn(200), B: rand.Intn(100)}
	reply = &service.Reply{}
	ctx = context.Background()
	err = c.Call(ctx, "Arith.Minus", args, reply)
	if err != nil {
		log.Println("err!!!" + err.Error())
	} else if reply.C != args.A-args.B {
		log.Printf("%d - %d != %d", args.A, args.B, reply.C)
	}

	args = service.Args{A: rand.Intn(200), B: rand.Intn(100)}
	reply = &service.Reply{}
	ctx = context.Background()
	err = c.Call(ctx, "Arith.Mul", args, reply)
	if err != nil {
		log.Println("err!!!" + err.Error())
	} else if reply.C != args.A*args.B {
		log.Printf("%d * %d != %d", args.A, args.B, reply.C)
	}

	args = service.Args{A: rand.Intn(200), B: rand.Intn(100)}
	reply = &service.Reply{}
	ctx = context.Background()
	err = c.Call(ctx, "Arith.Divide", args, reply)
	if args.B == 0 && err == nil {
		log.Println("err!!! didn't return errror!")
	} else if err != nil && err.Error() == "divided by 0" {
		//log.Println(err.Error())
	} else if err != nil {
		log.Println("err!!!" + err.Error())
	} else if reply.C != args.A/args.B {
		log.Printf("%d / %d != %d", args.A, args.B, reply.C)
	}
}
