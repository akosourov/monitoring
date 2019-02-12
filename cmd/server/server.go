package main

import (
	"context"
	"log"

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
