package main

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"

	"github.com/akosourov/monitoring/cmd"
	pb "github.com/akosourov/monitoring/grpc"
)

func main() {
	conn, err := grpc.Dial("localhost:8000", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Fail to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewMonitoringClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	urls, err := cmd.MakeURLs("./../sites.txt")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}

	for _, url := range urls {
		info, err := client.GetURLInfo(ctx, &pb.RequestURL{Url: url})
		var ms int
		if info != nil {
			ms = int(info.AvgLatency / 1000000)
		}
		log.Printf(url+": response: %+v ms: %d error: %v", info, ms, err)
	}

	log.Println()
	minLat, err := client.GetMinLatency(ctx, &pb.Empty{})
	log.Printf("GetMinLatency: response: %+v ms: %d error: %v", minLat, minLat.AvgLatency/1000000, err)

	log.Println()
	maxLat, err := client.GetMaxLatency(ctx, &pb.Empty{})
	log.Printf("GetMaxLatency: response: %+v ms: %d error: %v", maxLat, maxLat.AvgLatency/1000000, err)
}
