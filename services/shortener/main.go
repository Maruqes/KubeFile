package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/Maruqes/KubeFile/shared/proto/shortener"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

var (
	redisClient *redis.Client
	ctx         = context.Background()
)

type ShortenerService struct {
	shortener.UnimplementedShortenerServer
}

func (s *ShortenerService) ShortURL(ctx context.Context, req *shortener.ShortURLRequest) (*shortener.ShortURLResponse, error) {
	cur_uuid := uuid.NewString()
	ttl := 5 * 24 * time.Hour
	err := redisClient.Set(ctx, cur_uuid, req.OriginalURL, ttl).Err()
	if err != nil {
		return nil, fmt.Errorf("error storing short URL: %v", err)
	}
	return &shortener.ShortURLResponse{
		UUID: cur_uuid,
	}, nil
}

func (s *ShortenerService) ResolveURL(ctx context.Context, req *shortener.ResolveURLRequest) (*shortener.ResolveURLResponse, error) {
	val, err := redisClient.Get(ctx, req.UUID).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("URL not found for UUID: %s", req.UUID)
		}
		return nil, fmt.Errorf("error retrieving URL for UUID %s: %v", req.UUID, err)
	}
	log.Printf("Retrieved URL for UUID %s: %s", req.UUID, val)
	return &shortener.ResolveURLResponse{
		OriginalURL: val,
	}, nil
}

func connectRedis(ctx context.Context, addr string, attempts int, delay time.Duration) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   0,
	})
	for i := 0; i < attempts; i++ {
		if _, err := client.Ping(ctx).Result(); err == nil {
			return client, nil
		} else {
			log.Printf("Failed to connect to Redis (attempt %d/%d): %v", i+1, attempts, err)
		}
		time.Sleep(delay)
	}
	return nil, fmt.Errorf("could not connect to Redis at %s after %d attempts", addr, attempts)
}

func main() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "redis-service:6379"
	}
	client, err := connectRedis(ctx, redisAddr, 60, 2*time.Second)
	if err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}
	redisClient = client
	fmt.Println("Connected to Redis")

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", 50051))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	shortener.RegisterShortenerServer(grpcServer, &ShortenerService{})

	log.Println("Starting gRPC server on port 50051...")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
