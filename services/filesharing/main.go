package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	MinioImpl "github.com/Maruqes/KubeFile/services/filesharing/Minio"
	"github.com/Maruqes/KubeFile/shared/proto/filesharing"
	"github.com/minio/minio-go/v7"
	"google.golang.org/grpc"
)

var minioClient *minio.Client

type FilesharingService struct {
	filesharing.UnimplementedFileUploadServer
}

func (f *FilesharingService) UploadFile(ctx context.Context, req *filesharing.UploadFileRequest) (*filesharing.UploadFileResponse, error) {
	err := MinioImpl.ClearFile(ctx, minioClient, "ficheiros", req.FileName)
	if err != nil {
		return nil, fmt.Errorf("error clearing file: %v", err)
	}
	log.Println("Cleared file:", req.FileName)

	err = MinioImpl.AddChunkToFile(ctx, minioClient, "ficheiros", req.FileName, req.FileContent)
	if err != nil {
		return nil, fmt.Errorf("error uploading file: %v", err)
	}
	log.Printf("Added chunk to file %s", req.FileName)
	res := &filesharing.UploadFileResponse{
		FileName: req.FileName,
	}

	return res, nil
}

func (f *FilesharingService) AddChunk(ctx context.Context, req *filesharing.AddChunkRequest) (*filesharing.AddChunkResponse, error) {
	err := MinioImpl.AddChunkToFile(ctx, minioClient, "ficheiros", req.FileName, req.ChunkData)
	if err != nil {
		return nil, fmt.Errorf("error adding chunk to file: %v", err)
	}
	log.Printf("Added chunk to file %s", req.FileName)

	return &filesharing.AddChunkResponse{
		Success: true,
		Message: "Chunk added successfully",
	}, nil
}

func (f *FilesharingService) GetChunk(ctx context.Context, req *filesharing.GetChunkRequest) (*filesharing.GetChunkResponse, error) {

	// Build chunk object name based on index
	chunkObjectName := req.FileName + "_chunk_" + fmt.Sprintf("%d", req.ChunkIndex)

	//check if the chunk exists
	_, err := minioClient.StatObject(ctx, "ficheiros", chunkObjectName, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, fmt.Errorf("chunk %s does not exist", chunkObjectName)
		}
		return nil, fmt.Errorf("error checking chunk existence: %v", err)
	}

	object, err := minioClient.GetObject(ctx, "ficheiros", chunkObjectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting chunk from MinIO: %v", err)
	}
	defer object.Close()

	const maxChunkSize = 30 * 1024 * 1024 // 30MB
	chunkData := make([]byte, maxChunkSize)

	n, err := object.Read(chunkData)
	if err != nil && err.Error() != "EOF" {
		return nil, fmt.Errorf("error reading chunk data: %v", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("no data read from chunk %s", chunkObjectName)
	}

	// Trim the slice to actual data size
	chunkData = chunkData[:n]

	// Check if this is the last chunk by looking for the next chunk
	nextChunkName := req.FileName + "_chunk_" + fmt.Sprintf("%d", req.ChunkIndex+1)
	_, err = minioClient.StatObject(ctx, "ficheiros", nextChunkName, minio.StatObjectOptions{})
	isLastChunk := err != nil // If next chunk doesn't exist, this is the last chunk

	return &filesharing.GetChunkResponse{
		ChunkData:   chunkData,
		ChunkIndex:  req.ChunkIndex,
		IsLastChunk: isLastChunk,
	}, nil
}

func (f *FilesharingService) GetStorageInfo(ctx context.Context, req *filesharing.GetStorageInfoRequest) (*filesharing.GetStorageInfoResponse, error) {
	storageInfo, err := MinioImpl.GetStorageLimitsData(ctx, minioClient)
	if err != nil {
		return nil, fmt.Errorf("error getting storage info: %v", err)
	}

	return &storageInfo, nil
}

func setupBucket(minioClient *minio.Client) error {
	if minioClient == nil {
		return fmt.Errorf("MinIO client is not available")
	}

	bucketName := "ficheiros"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("error checking bucket: %v", err)
	}

	if !exists {
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("error creating bucket: %v", err)
		}
		log.Printf("✅ Bucket created: %s", bucketName)
	} else {
		log.Printf("ℹ️  Bucket already exists: %s", bucketName)
	}

	return nil
}

func main() {
	minioClient = MinioImpl.InitializeMinIO()
	if minioClient == nil {
		log.Fatalf("failed to initialize MinIO client")
	}

	if err := setupBucket(minioClient); err != nil {
		log.Fatalf("bucket setup failed: %v", err)
	}

	// Start cleanup goroutine to remove objects older than 5 days
	go func() {
		ttl := 5 * 24 * time.Hour
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			cleanStartCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			objectsCh := minioClient.ListObjects(cleanStartCtx, "ficheiros", minio.ListObjectsOptions{Recursive: true})
			for obj := range objectsCh {
				if obj.Err != nil {
					log.Printf("Warning: error listing object during cleanup: %v", obj.Err)
					continue
				}
				if time.Since(obj.LastModified) > ttl {
					err := minioClient.RemoveObject(cleanStartCtx, "ficheiros", obj.Key, minio.RemoveObjectOptions{})
					if err != nil {
						log.Printf("Failed to remove expired object %s: %v", obj.Key, err)
					} else {
						log.Printf("Removed expired object: %s", obj.Key)
					}
				}
			}
			cancel()
			<-ticker.C
		}
	}()

	// Create a TCP listener on port 50052
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", 50052))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	maxMsgSize := 31 * 1024 * 1024
	fmt.Println("Setting maximum message size to:", maxMsgSize)
	fmt.Println("Setting maximum message size to:", maxMsgSize)

	var opts []grpc.ServerOption
	opts = append(opts, grpc.MaxRecvMsgSize(maxMsgSize))
	opts = append(opts, grpc.MaxSendMsgSize(maxMsgSize))

	grpcServer := grpc.NewServer(opts...)
	filesharing.RegisterFileUploadServer(grpcServer, &FilesharingService{})

	log.Println("Starting gRPC server on port 50052...")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
