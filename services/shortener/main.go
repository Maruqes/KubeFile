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
	err := redisClient.Set(ctx, cur_uuid, req.OriginalURL, 0).Err()
	if err != nil {
		log.Fatalf("Error setting key: %v", err)
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

func main() {
	// Start gRPC server first, then handle Redis connection asynchronously
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", 50051))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	shortener.RegisterShortenerServer(grpcServer, &ShortenerService{})

	// Start Redis connection in a goroutine
	go func() {
		// Get Redis address from environment variable or use default
		redisAddr := os.Getenv("REDIS_ADDR")
		if redisAddr == "" {
			redisAddr = "redis-service:6379"
		}

		redisClient = redis.NewClient(&redis.Options{
			Addr: redisAddr,
			DB:   0,
		})

		// Retry Redis connection with timeout
		for i := 0; i < 10; i++ {
			pong, err := redisClient.Ping(ctx).Result()
			if err == nil {
				fmt.Println("Connected to Redis:", pong)
				break
			}
			log.Printf("Failed to connect to Redis (attempt %d/10): %v", i+1, err)
			time.Sleep(2 * time.Second)
			if i == 9 {
				log.Printf("Warning: Could not connect to Redis after 10 attempts, continuing without Redis")
				break
			}
		}
	}()

	log.Println("Starting gRPC server on port 50051...")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
