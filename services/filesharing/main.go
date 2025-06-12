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
	err := uploadFileToS3(minioClient, "ficheiros", req.FileName, req.FileContent)
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
	//5.5MB chunk size
	chunkSize := int64(5.5 * 1024 * 1024) // 5.5MB
	start := int64(req.ChunkIndex) * chunkSize
	end := start + chunkSize - 1

	// Get the object info to check file size
	objInfo, err := minioClient.StatObject(ctx, "ficheiros", req.FileName, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting file info: %v", err)
	}

	if start >= objInfo.Size {
		return nil, fmt.Errorf("chunk index out of range")
	}
	if end >= objInfo.Size {
		end = objInfo.Size - 1
	}

	// Get only the specific chunk range
	opts := minio.GetObjectOptions{}
	err = opts.SetRange(start, end)
	if err != nil {
		return nil, fmt.Errorf("error setting range: %v", err)
	}

	object, err := minioClient.GetObject(ctx, "ficheiros", req.FileName, opts)
	if err != nil {
		return nil, fmt.Errorf("error getting chunk from MinIO: %v", err)
	}
	defer object.Close()

	// Read the chunk data
	chunkData := make([]byte, end-start+1)
	_, err = object.Read(chunkData)
	if err != nil && err.Error() != "EOF" {
		return nil, fmt.Errorf("error reading chunk: %v", err)
	}

	return &filesharing.GetChunkResponse{
		ChunkData: chunkData,
	}, nil
}

func addChunkToFile(ctx context.Context, minioClient *minio.Client, bucketName, objectName string, chunkData []byte) error {
	log.Printf("📦 Starting addChunkToFile for object: %s, chunk size: %d bytes", objectName, len(chunkData))

	// Check if the object exists
	_, err := minioClient.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		// If object doesn't exist, create it with the first chunk
		log.Printf("🆕 Object doesn't exist, creating new file with first chunk")
		reader := bytes.NewReader(chunkData)
		info, err := minioClient.PutObject(ctx, bucketName, objectName, reader, int64(len(chunkData)), minio.PutObjectOptions{})
		if err != nil {
			log.Printf("❌ Failed to create new file: %v", err)
			return fmt.Errorf("error creating new file in MinIO: %v", err)
		}
		log.Printf("✅ New file created successfully: %s (size: %d bytes)", info.Key, info.Size)
		return nil
	}

	log.Printf("📄 Object exists, appending chunk to existing file: %s", objectName)

	// Use ComposeObject to append chunk to existing object
	srcInfo := minio.CopySrcOptions{
		Bucket: bucketName,
		Object: objectName,
	}

	// Upload the new chunk as a temporary object
	tempObjectName := objectName + "_temp_chunk"
	log.Printf("⏳ Uploading temporary chunk: %s", tempObjectName)
	reader := bytes.NewReader(chunkData)
	_, err = minioClient.PutObject(ctx, bucketName, tempObjectName, reader, int64(len(chunkData)), minio.PutObjectOptions{})
	if err != nil {
		log.Printf("❌ Failed to upload temporary chunk: %v", err)
		return fmt.Errorf("error uploading temporary chunk: %v", err)
	}
	log.Printf("✅ Temporary chunk uploaded successfully: %s", tempObjectName)

	// Compose the original object with the new chunk
	tempSrcInfo := minio.CopySrcOptions{
		Bucket: bucketName,
		Object: tempObjectName,
	}

	dst := minio.CopyDestOptions{
		Bucket: bucketName,
		Object: objectName,
	}

	log.Printf("🔄 Composing objects: %s + %s -> %s", objectName, tempObjectName, objectName)
	_, err = minioClient.ComposeObject(ctx, dst, srcInfo, tempSrcInfo)
	if err != nil {
		log.Printf("❌ Failed to compose objects: %v", err)
		// Clean up temp object on failure
		removeErr := minioClient.RemoveObject(ctx, bucketName, tempObjectName, minio.RemoveObjectOptions{})
		if removeErr != nil {
			log.Printf("❌ Failed to clean up temporary object after compose failure: %v", removeErr)
		} else {
			log.Printf("🧹 Cleaned up temporary object after compose failure: %s", tempObjectName)
		}
		return fmt.Errorf("error composing objects: %v", err)
	}
	log.Printf("✅ Objects composed successfully")

	// Clean up temporary object
	log.Printf("🧹 Cleaning up temporary object: %s", tempObjectName)
	err = minioClient.RemoveObject(ctx, bucketName, tempObjectName, minio.RemoveObjectOptions{})
	if err != nil {
		log.Printf("⚠️ Warning: failed to remove temporary object: %v", err)
	} else {
		log.Printf("✅ Temporary object removed successfully: %s", tempObjectName)
	}

	log.Printf("✅ Chunk appended successfully to: %s", objectName)
	return nil
}

func uploadFileToS3(minioClient *minio.Client, bucketName, objectName string, fileData []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a reader from the byte data
	reader := bytes.NewReader(fileData)

	// Upload the file to MinIO
	info, err := minioClient.PutObject(ctx, bucketName, objectName, reader, int64(len(fileData)), minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("error uploading file to MinIO: %v", err)
	}

	log.Printf("File uploaded successfully: %s (size: %d bytes)", info.Key, info.Size)
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
	maxMsgSize := 6 * 1024 * 1024 // 6MB
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
