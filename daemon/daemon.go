package main

import (
	"flag"
	"github.com/xhebox/chrootd/api/container"
	//"github.com/xhebox/chrootd/api/containerpool"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/sevlyar/go-daemon"
	. "github.com/xhebox/chrootd/common"
	"google.golang.org/grpc"
)




var (
	signal *string
	stop   = make(chan struct{})
)


func termHandler(sig os.Signal) error {
	stop <- struct{}{}
	log.Println("terminate")
	return daemon.ErrStop
}

func main() {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	signal = fs.String("s", "", `stop — shutdown`)

	daemonConf := NewDaemonConfig()
	daemonConf.SetFlag(fs)

	containerGroup := make(map[string]*SingleContainer)

	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatalln(err)
	}
	daemon.AddCommand(daemon.StringFlag(signal, "stop"), syscall.SIGINT, termHandler)
	daemonConf.LoadEnv()

	if err := daemonConf.ParseIni(); err != nil {
		log.Fatalln(err)
	}

	cntxt := &daemon.Context{
		PidFileName: daemonConf.PidFileName,
		PidFilePerm: daemonConf.PidFilePerm,
		LogFileName: daemonConf.LogFileName,
		LogFilePerm: daemonConf.LogFilePerm,
		WorkDir:     daemonConf.WorkDir,
		Umask:       027,
		Args:        []string{"[go-daemon sample]"},
	}

	if len(daemon.ActiveFlags()) > 0 {
		d, err := cntxt.Search()
		if err != nil {
			log.Fatalf("Unable send signal to the daemon: %v\n", err)
		}
		daemon.SendCommands(d)
		return
	}

	d, err := cntxt.Reborn()
	if err != nil {
		log.Fatalf("Unable to run: %v\n", err)
	}
	if d != nil {
		return
	}
	defer cntxt.Release()

	log.Println("daemon started")

	lis, err := daemonConf.GrpcConn.Listen()
	if err != nil {
		log.Fatalf("server is unable to listen: %v\n", err)
	}
	log.Printf("server listening in %v, %v", daemonConf.GrpcConn.Addr, daemonConf.GrpcConn.NetWorkType)
	defer lis.Close()

	//poolGrpcServer := grpc.NewServer()
	//poolSrv := NewPoolServer()
	//containerpool.RegisterContainerPoolServer(poolGrpcServer, poolSrv)

	containerGrpcServer := grpc.NewServer()
	containerSrv := NewContainerServer()
	containerSrv.group = &containerGroup
	Container.RegisterContainerServer(containerGrpcServer, containerSrv)

	go func() {
		if err := containerGrpcServer.Serve(lis); err != nil {
			log.Printf("grpc: ContainerServer server failed to serve: %v\n", err)
			stop <- struct{}{}
		}
	}()

	//go func() {
	//	if err := poolGrpcServer.Serve(lis); err != nil {
	//		log.Printf("grpc: pool server failed to serve: %v\n", err)
	//		stop <- struct{}{}
	//	}
	//}()

	go func() {
	loop:
		for {
			time.Sleep(time.Second)
			select {
			case <-stop:
				//poolGrpcServer.GracefulStop()
				containerGrpcServer.GracefulStop()
				break loop
			default:
			}
		}
	}()

	if err = daemon.ServeSignals(); err != nil {
		log.Printf("Fail to serve signals: %v\n", err)
	}
}
