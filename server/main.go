package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	pb "github.com/akosourov/monitoring/grpc"
	"github.com/akosourov/monitoring/storage"
)

type monitoringServer struct {
	db storage.Storage
}

func (s *monitoringServer) GetURLInfo(ctx context.Context, req *pb.RequestURL) (*pb.ResponseInfo, error) {
	lat, err := s.db.GetLastLatency(req.Url)
	if err != nil {
		log.Println("[WARN] storage error", err)
		return nil, err
	}

	resp := new(pb.ResponseInfo)
	if lat >= 0 {
		resp.IsAvailable = true
		resp.AvgLatency = lat
	}

	return resp, nil
}

func (s *monitoringServer) GetMaxLatency(ctx context.Context, _ *pb.Empty) (*pb.ResponseLatency, error) {
	url, lat, err := s.db.GetMaxLatency()
	if err != nil {
		log.Println("[WARN] storage error", err)
		return nil, err
	}

	resp := new(pb.ResponseLatency)
	resp.Url = url
	resp.AvgLatency = lat
	return resp, nil
}

func (s *monitoringServer) GetMinLatency(ctx context.Context, _ *pb.Empty) (*pb.ResponseLatency, error) {
	url, lat, err := s.db.GetMinLatency()
	if err != nil {
		log.Println("[WARN] storage error", err)
		return nil, err
	}

	resp := new(pb.ResponseLatency)
	resp.Url = url
	resp.AvgLatency = lat
	return resp, nil
}

func (s *monitoringServer) putInitialData() error {
	err := s.db.PutLatency("https://ya.ru", 200)
	if err != nil {
		return err
	}
	err = s.db.PutLatency("https://google.com", 300)
	if err != nil {
		return err
	}
	err = s.db.PutLatency("https://google.com", 130)
	if err != nil {
		return err
	}
	return nil
}

func newServer(dbpath string) *monitoringServer {
	return &monitoringServer{
		db: storage.NewBoltStorage(dbpath),
	}
}

var (
	address = flag.String("address", "localhost:8000", "host:port")
	dbpath  = flag.String("dbpath", "./bolt.db", "path to db file")
)

func main() {
	flag.Parse()

	lis, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	monServer := newServer(*dbpath)
	defer func() {
		log.Println("[INFO] close db")
		if err := monServer.db.Close(); err != nil {
			log.Println("[WARN] failed to close db:", err)
		}
	}()
	if err := monServer.putInitialData(); err != nil {
		log.Fatalf("[WARN] Failed initial data: %v", err)
	}
	pb.RegisterMonitoringServer(grpcServer, monServer)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-interrupt
		log.Println("[WARN] signal", sig)
		grpcServer.GracefulStop()
	}()

	log.Println("[INFO] Start grpc server")
	grpcServer.Serve(lis)
}
