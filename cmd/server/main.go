package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"

	pb "github.com/akosourov/monitoring/grpc"
	"github.com/akosourov/monitoring/poller"
	"github.com/akosourov/monitoring/storage"
)

func main() {
	sitesfname := flag.String("sites", "./../sites.txt", "Filename with list of sites")
	interval := flag.String("interval", "10s", "Poll interval")
	address := flag.String("address", "localhost:8000", "host:port")
	dbpath := flag.String("dbpath", "./../bolt.db", "path to db file")
	workers := flag.Int("workers", 8, "pool of workers")
	flag.Parse()

	// читаем файл с сайтами и конвертируем их в https url
	urls, err := makeURLs(*sitesfname)
	if err != nil {
		log.Fatalf("[ERROR] Cant open file: %v", err)
	}

	dur, err := time.ParseDuration(*interval)
	if err != nil {
		log.Fatalf("[ERROR] Bad interval param: %v", err)
	}

	// создаем подключение к бд
	db := storage.NewBoltStorage(*dbpath)
	defer func() {
		if p := recover(); p != nil {
			log.Printf("[ERROR] catch panic: %v", p)
		}
		log.Println("[INFO] program exit")
		if err := db.Close(); err != nil {
			log.Println("[ERROR] failed to close db:", err)
		}
	}()

	// Запускаем поллер
	pol := poller.Poller{
		URLs:     urls,
		Interval: dur,
		Timeout:  2 * time.Second,
		DB:       db,
		Workers:  *workers,
	}
	go func() {
		pol.Start()
	}()

	// Запускаем grpc сервер
	lis, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatalf("[ERROR] Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	monServer := monitoringServer{db}
	pb.RegisterMonitoringServer(grpcServer, &monServer)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-interrupt
		log.Println("[WARN] signal", sig)
		pol.Stop()
		grpcServer.GracefulStop()
	}()

	log.Println("[INFO] Start grpc server")
	grpcServer.Serve(lis)
}

func makeURLs(fname string) ([]string, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	urls := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = "https://" + url
		}
		urls = append(urls, url)
	}
	if scanner.Err() != nil {
		return nil, err
	}

	return urls, nil
}
