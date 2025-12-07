package MinioImpl

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Maruqes/KubeFile/shared/proto/filesharing"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func GetChuckLastIndex(ctx context.Context, minioClient *minio.Client, bucketName, objectName string) (int64, error) {
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
		return -1, nil // No chunks found, return -1 so first chunk will be index 0
	}
	return lastIndex, nil
}

func AddChunkToFile(ctx context.Context, minioClient *minio.Client, bucketName, objectName string, chunkData []byte) error {
	last_index, err := GetChuckLastIndex(ctx, minioClient, bucketName, objectName)
	if err != nil {
		return fmt.Errorf("error getting last chunk index: %v", err)
	}

	chunk_add := objectName + "_chunk_" + fmt.Sprintf("%d", last_index+1)
	reader := bytes.NewReader(chunkData)
	_, err = minioClient.PutObject(ctx, bucketName, chunk_add, reader, int64(len(chunkData)), minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("error uploading temporary chunk: %v", err)
	}
	return nil
}

func ClearFile(ctx context.Context, minioClient *minio.Client, bucketName, objectName string) error {
	objectsCh := minioClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Prefix: objectName,
	})

	for obj := range objectsCh {
		minioClient.RemoveObject(ctx, bucketName, obj.Key, minio.RemoveObjectOptions{})
		if obj.Err != nil {
			return fmt.Errorf("error listing objects: %v", obj.Err)
		}
	}
	return nil
}

func GetStorageLimitsData(ctx context.Context, minioClient *minio.Client) (filesharing.GetStorageInfoResponse, error) {
	//get Gigas ocupied in minio
	buckets, err := minioClient.ListBuckets(ctx)
	if err != nil {
		return filesharing.GetStorageInfoResponse{}, fmt.Errorf("error listing buckets: %v", err)
	}
	var totalSize int64 = 0
	for _, bucket := range buckets {
		objectsCh := minioClient.ListObjects(ctx, bucket.Name, minio.ListObjectsOptions{})
		for obj := range objectsCh {
			if obj.Err != nil {
				return filesharing.GetStorageInfoResponse{}, fmt.Errorf("error listing objects: %v", obj.Err)
			}
			totalSize += obj.Size
		}
	}
	// Convert total size to gigabytes
	gigasOccupied := int(totalSize / (1024 * 1024 * 1024))
	gigasLimit := 200 // Assuming a hardcoded limit of 200 GB
	return filesharing.GetStorageInfoResponse{
		TotalSize: int64(gigasLimit),
		UsedSize:  int64(gigasOccupied),
	}, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func InitializeMinIO(ctx context.Context) (*minio.Client, error) {
	endpoint := getEnv("MINIO_ENDPOINT", "minio-service.minio:9000")
	accessKey := getEnv("MINIO_ACCESS_KEY", "MINIO_ACCESS_KEY")
	secretKey := getEnv("MINIO_SECRET_KEY", "MINIO_SECRET_KEY")
	useSSL := os.Getenv("MINIO_USE_SSL") == "true"

	const attempts = 70
	const delay = 2 * time.Second

	log.Printf("Attempting to connect to MinIO at: %s", endpoint)

	var minioClient *minio.Client
	var err error

	for i := 0; i < attempts; i++ {
		minioClient, err = minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure: useSSL,
		})
		if err != nil {
			log.Printf("Failed to create MinIO client (attempt %d/%d): %v", i+1, attempts, err)
			time.Sleep(delay)
			continue
		}

		connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		buckets, err := minioClient.ListBuckets(connectCtx)
		cancel()

		if err != nil {
			log.Printf("Failed to connect to MinIO (attempt %d/%d): %v", i+1, attempts, err)
			time.Sleep(delay)
			continue
		}

		log.Printf("✅ Connected to MinIO successfully! Found %d buckets", len(buckets))
		return minioClient, nil
	}

	return nil, fmt.Errorf("could not connect to MinIO after %d attempts", attempts)
}
