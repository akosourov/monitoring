package main

import (
	"context"
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
	resp := new(pb.ResponseLatency)
	return resp, nil
}

func (s *monitoringServer) GetMinLatency(ctx context.Context, _ *pb.Empty) (*pb.ResponseLatency, error) {
	resp := new(pb.ResponseLatency)
	return resp, nil
}

func newServer(dbpath string, urls []string) *monitoringServer {
	return &monitoringServer{
		db: storage.NewBoltStorage(dbpath, urls),
	}
}

func main() {
	lis, err := net.Listen("tcp", ":8000")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	monServer := newServer("./bolt.db", []string{"ya.ru", "google.com"})
	defer monServer.db.Close()
	pb.RegisterMonitoringServer(grpcServer, monServer)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-interrupt
		log.Println("[WARN] signal", sig)
		grpcServer.GracefulStop()
	}()

	grpcServer.Serve(lis)
}
