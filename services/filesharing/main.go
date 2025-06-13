package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/Maruqes/KubeFile/shared/proto/filesharing"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"google.golang.org/grpc"
)

var (
	user      string
	pass      string
	server    string
	port      string
	remoteDir string
)
var minioClient *minio.Client

type FilesharingService struct {
	filesharing.UnimplementedFileUploadServer
}

func (f *FilesharingService) UploadFile(ctx context.Context, req *filesharing.UploadFileRequest) (*filesharing.UploadFileResponse, error) {
	err := addChunkToFile(ctx, minioClient, "ficheiros", req.FileName, req.FileContent)
	if err != nil {
		return nil, fmt.Errorf("error uploading file: %v", err)
	}
	res := &filesharing.UploadFileResponse{
		FileURL:  req.CurrentUrl,
		FileName: req.FileName,
	}

	return res, nil
}

func (f *FilesharingService) AddChunk(ctx context.Context, req *filesharing.AddChunkRequest) (*filesharing.AddChunkResponse, error) {
	err := addChunkToFile(ctx, minioClient, "ficheiros", req.FileName, req.ChunkData)
	if err != nil {
		return nil, fmt.Errorf("error adding chunk to file: %v", err)
	}

	return &filesharing.AddChunkResponse{
		Success: true,
		Message: "Chunk added successfully",
	}, nil
}

func (f *FilesharingService) GetFile(ctx context.Context, req *filesharing.GetFileRequest) (*filesharing.GetFileResponse, error) {
	fileData, err := returnFileFromS3(minioClient, "ficheiros", req.FileName)
	if err != nil {
		return nil, fmt.Errorf("error retrieving file: %v", err)
	}

	res := &filesharing.GetFileResponse{
		FileContent: fileData,
		FileName:    req.FileName,
	}

	return res, nil
}

func (f *FilesharingService) GetChunk(ctx context.Context, req *filesharing.GetChunkRequest) (*filesharing.GetChunkResponse, error) {
	// Build chunk object name based on index
	chunkObjectName := req.FileName + "_chunk_" + fmt.Sprintf("%d", req.ChunkIndex)

	// Get the chunk object from MinIO
	object, err := minioClient.GetObject(ctx, "ficheiros", chunkObjectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting chunk from MinIO: %v", err)
	}
	defer object.Close()

	// Read the chunk data with 30MB limit
	const maxChunkSize = 30 * 1024 * 1024 // 30MB
	chunkData := make([]byte, 0, maxChunkSize)
	buf := make([]byte, 32*1024) // 32KB buffer for better performance
	totalRead := 0
	
	for {
		n, err := object.Read(buf)
		if n > 0 {
			// Check if adding this data would exceed the limit
			if totalRead+n > maxChunkSize {
				remaining := maxChunkSize - totalRead
				if remaining > 0 {
					chunkData = append(chunkData, buf[:remaining]...)
				}
				break
			}
			chunkData = append(chunkData, buf[:n]...)
			totalRead += n
		}
		if err != nil {
			if err.Error() == "EOF" {
				break // End of file reached
			}
			return nil, fmt.Errorf("error reading chunk: %v", err)
		}
	}

	// Check if this is the last chunk by looking for the next chunk
	nextChunkName := req.FileName + "_chunk_" + fmt.Sprintf("%d", req.ChunkIndex+1)
	_, err = minioClient.StatObject(ctx, "ficheiros", nextChunkName, minio.StatObjectOptions{})
	isLastChunk := err != nil // If next chunk doesn't exist, this is the last chunk

	log.Printf("Chunk retrieved successfully: %s (size: %d bytes, isLastChunk: %v)", chunkObjectName, len(chunkData), isLastChunk)
	return &filesharing.GetChunkResponse{
		ChunkData:   chunkData,
		ChunkIndex:  req.ChunkIndex,
		IsLastChunk: isLastChunk,
	}, nil
}

func getChuckLastIndex(ctx context.Context, minioClient *minio.Client, bucketName, objectName string) (int64, error) {
	//get all object names starting with the objectName prefix
	objectsCh := minioClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Prefix: objectName,
	})
	var lastIndex int64 = -1
	for obj := range objectsCh {
		if obj.Err != nil {
			return -1, fmt.Errorf("error listing objects: %v", obj.Err)
		}
		// Check if the object name starts with the objectName prefix
		if len(obj.Key) > len(objectName) && obj.Key[:len(objectName)] == objectName {
			// Extract the index from the object name
			var index int64
			_, err := fmt.Sscanf(obj.Key[len(objectName):], "_chunk_%d", &index)
			if err != nil {
				log.Printf("⚠️  Warning: Failed to parse index from object name %s: %v", obj.Key, err)
				continue
			}
			if index > lastIndex {
				lastIndex = index
			}
		}
	}
	if lastIndex == -1 {
		log.Printf("ℹ️  No chunks found for object: %s", objectName)
		return -1, nil // No chunks found, return -1 so first chunk will be index 0
	}
	log.Printf("✅ Last chunk index for object %s is: %d", objectName, lastIndex)
	return lastIndex, nil
}

func addChunkToFile(ctx context.Context, minioClient *minio.Client, bucketName, objectName string, chunkData []byte) error {
	log.Printf("📦 Starting addChunkToFile for object: %s, chunk size: %d bytes", objectName, len(chunkData))

	last_index, err := getChuckLastIndex(ctx, minioClient, bucketName, objectName)
	if err != nil {
		log.Printf("❌ Failed to get last chunk index: %v", err)
		return fmt.Errorf("error getting last chunk index: %v", err)
	}

	chunk_add := objectName + "_chunk_" + fmt.Sprintf("%d", last_index+1)
	log.Printf("⏳ Uploading temporary chunk: %s", chunk_add)
	reader := bytes.NewReader(chunkData)
	_, err = minioClient.PutObject(ctx, bucketName, chunk_add, reader, int64(len(chunkData)), minio.PutObjectOptions{})
	if err != nil {
		log.Printf("❌ Failed to upload temporary chunk: %v", err)
		return fmt.Errorf("error uploading temporary chunk: %v", err)
	}
	log.Printf("✅ Temporary chunk uploaded successfully: %s", chunk_add)

	log.Printf("✅ Chunk appended successfully to: %s", objectName)
	return nil
}

func returnFileFromS3(minioClient *minio.Client, bucketName, objectName string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get the object from MinIO
	object, err := minioClient.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting file from MinIO: %v", err)
	}
	defer object.Close()

	// Read the object data into a byte slice
	fileData := make([]byte, 0)
	buf := make([]byte, 1024) // Buffer size of 1KB
	for {
		n, err := object.Read(buf)
		if n > 0 {
			fileData = append(fileData, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break // End of file reached
			}
			return nil, fmt.Errorf("error reading file from MinIO: %v", err)
		}
	}

	log.Printf("File retrieved successfully: %s", objectName)
	return fileData, nil
}

func initializeMinIO() *minio.Client {
	// MinIO configuration with hardcoded values
	endpoint := "minio-service.minio:9000"
	accessKey := "MINIO_ACCESS_KEY"
	secretKey := "MINIO_SECRET_KEY"
	useSSL := false

	log.Printf("Attempting to connect to MinIO at: %s", endpoint)

	// Create MinIO client with retry logic
	var minioClient *minio.Client
	var err error

	for i := 0; i < 10; i++ {
		minioClient, err = minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure: useSSL,
		})
		if err != nil {
			log.Printf("Failed to create MinIO client (attempt %d/10): %v", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}

		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		buckets, err := minioClient.ListBuckets(ctx)
		cancel()

		if err != nil {
			log.Printf("Failed to connect to MinIO (attempt %d/10): %v", i+1, err)
			time.Sleep(2 * time.Second)
			continue
		}

		log.Printf("✅ Connected to MinIO successfully! Found %d buckets", len(buckets))
		return minioClient
	}

	log.Printf("⚠️  Warning: Could not connect to MinIO after 10 attempts. Service will continue without MinIO.")
	return nil
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
	minioClient = initializeMinIO()

	if minioClient != nil {
		if err := setupBucket(minioClient); err != nil {
			log.Printf("Warning: Bucket setup failed: %v", err)
		}
	}

	// Create a TCP listener on port 50052
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", 50052))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Set maximum message size to 6MB for both send and receive
	maxMsgSize := 31 * 1024 * 1024 // 6MB
	fmt.Println("Setting maximum message size to:", maxMsgSize)
	fmt.Println("Setting maximum message size to:", maxMsgSize)
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
