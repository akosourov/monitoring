package main

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"

	pb "github.com/akosourov/monitoring/grpc"
)

func main() {
	conn, err := grpc.Dial("localhost:8000", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Fail to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewMonitoringClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := client.GetURLInfo(ctx, &pb.RequestURL{Url: "https://google.com"})
	if err != nil {
		log.Println("Error ", err)
	}
	log.Printf("Response:  %+v", info)
}
