package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Maruqes/KubeFile/shared/proto/filesharing"
	"github.com/Maruqes/KubeFile/shared/proto/shortener"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func isValidURL(url string, trying ...bool) string {
	// Simple URL validation logic
	if len(url) < 5 || (len(url) > 2083) || !((url[:4] == "http") || (url[:5] == "https")) {
		if len(trying) == 0 || !trying[0] {
			url = "https://" + url
			return isValidURL(url, true)
		}
		return ""
	}
	return url
}

func askForShortURL(w http.ResponseWriter, r *http.Request, client shortener.ShortenerClient) {
	//get var url from GET request
	url := r.URL.Query().Get("url")

	url = isValidURL(url)
	if url == "" {
		http.Error(w, "URL inválida", http.StatusBadRequest)
		return
	}

	if url == "" {
		http.Error(w, "URL não fornecida", http.StatusBadRequest)
		return
	}

	url_final, err := client.ShortURL(r.Context(), &shortener.ShortURLRequest{
		OriginalURL: url,
	})
	if err != nil {
		http.Error(w, "Erro ao encurtar URL", http.StatusInternalServerError)
		return
	}
	w.Write([]byte(url_final.UUID))
	return
}

func getMainUrl(w http.ResponseWriter, r *http.Request, client shortener.ShortenerClient) {
	//get var url from GET request
	user_uuid := r.URL.Query().Get("uuid")
	if user_uuid == "" {
		http.Error(w, "uuid não fornecida", http.StatusBadRequest)
		return
	}

	resp, err := client.ResolveURL(r.Context(), &shortener.ResolveURLRequest{
		UUID: user_uuid,
	})
	if err != nil {
		http.Error(w, "Erro ao resolver URL", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Location", resp.OriginalURL)
	w.WriteHeader(http.StatusFound)
	fmt.Fprintf(w, "Redirecting to %s...", resp.OriginalURL)
}

func handleUploadFile(w http.ResponseWriter, r *http.Request, client filesharing.FileUploadClient) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	filename := r.URL.Query().Get("filename")
	if filename == "" {
		http.Error(w, "Filename not provided", http.StatusBadRequest)
		return
	}

	// Read file content from request body
	contentFile, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading file content", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	//get current url
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := scheme + "://" + r.Host

	res, err := client.UploadFile(r.Context(), &filesharing.UploadFileRequest{
		FileName:    filename,
		FileContent: []byte(contentFile),
		CurrentUrl:  baseURL + "/download/" + filename,
	})
	if err != nil {
		fmt.Printf("Error uploading file: %v\n", err)
		http.Error(w, "Erro ao fazer upload do ficheiro", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "File uploaded successfully: %s\n", res.FileName)
}

func handleUploadChuck(w http.ResponseWriter, r *http.Request, client filesharing.FileUploadClient) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	filename := r.URL.Query().Get("filename")
	if filename == "" {
		http.Error(w, "Filename not provided", http.StatusBadRequest)
		return
	}

	// Read file content from request body
	contentFile, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading file content", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	res, err := client.AddChunk(r.Context(), &filesharing.AddChunkRequest{
		FileName:  filename,
		ChunkData: []byte(contentFile),
	})
	if err != nil {
		fmt.Printf("Error uploading file: %v\n", err)
		http.Error(w, "Erro ao fazer upload do ficheiro", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Chunk uploaded successfully: %s\nMessage: %s", filename, res.Message)
}

func handleGetStorageInfo(w http.ResponseWriter, r *http.Request, client filesharing.FileUploadClient) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	res, err := client.GetStorageInfo(r.Context(), &filesharing.GetStorageInfoRequest{})
	if err != nil {
		http.Error(w, "Erro ao obter informações de armazenamento", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"totalSize": %d, "usedSize": %d}`, res.TotalSize, res.UsedSize)
}

func handleGetFileChunk(w http.ResponseWriter, r *http.Request, client filesharing.FileUploadClient) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	fileName := r.URL.Query().Get("fileName")
	if fileName == "" {
		http.Error(w, "File name not provided", http.StatusBadRequest)
		return
	}
	chunkIndex := r.URL.Query().Get("chunkIndex")
	if chunkIndex == "" {
		http.Error(w, "Chunk index not provided", http.StatusBadRequest)
		return
	}

	chunkIndexInt, err := strconv.ParseInt(chunkIndex, 10, 32)
	if err != nil {
		http.Error(w, "Invalid chunk index", http.StatusBadRequest)
		return
	}

	res, err := client.GetChunk(r.Context(), &filesharing.GetChunkRequest{
		FileName:   fileName,
		ChunkIndex: int32(chunkIndexInt),
	})

	if err != nil {
		http.Error(w, "Erro ao obter chunk do ficheiro", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	w.Header().Set("Content-Type", "application/octet-stream")
	// Add custom header to indicate if this is the last chunk
	if res.IsLastChunk {
		w.Header().Set("X-Is-Last-Chunk", "true")
	} else {
		w.Header().Set("X-Is-Last-Chunk", "false")
	}
	w.Write(res.ChunkData)
}

func serveUnifiedPage(w http.ResponseWriter, r *http.Request) {
	// Get the absolute path to the static file
	staticDir := filepath.Join(".", "static")
	filePath := filepath.Join(staticDir, "filesharing.html")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("File not found: %s", filePath)
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}

	// Set content type for HTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, filePath)
}

func main() {
	maxMsgSize := 31 * 1024 * 1024 // 6MB

	// Setup shortener connection
	shortenerAddr := "shortener-service:50051"
	shortenerConn, err := grpc.NewClient(shortenerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize), grpc.MaxCallSendMsgSize(maxMsgSize)))
	if err != nil {
		log.Fatalf("Failed to connect to shortener service: %v", err)
	}
	defer shortenerConn.Close()
	shortenerClient := shortener.NewShortenerClient(shortenerConn)

	// Setup filesharing connection
	filesharingAddr := "filesharing-service:50052"
	filesharingConn, err := grpc.NewClient(filesharingAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize), grpc.MaxCallSendMsgSize(maxMsgSize)))
	if err != nil {
		log.Fatalf("Failed to connect to filesharing service: %v", err)
	}
	defer filesharingConn.Close()
	filesharingClient := filesharing.NewFileUploadClient(filesharingConn)

	// Configure HTTP routes
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app", http.StatusFound)
	})

	http.HandleFunc("/short", func(w http.ResponseWriter, r *http.Request) {
		askForShortURL(w, r, shortenerClient)
	})

	http.HandleFunc("/geturl", func(w http.ResponseWriter, r *http.Request) {
		getMainUrl(w, r, shortenerClient)
	})

	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}
		handleUploadFile(w, r, filesharingClient)
	})

	http.HandleFunc("/upload-chunk", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			handleUploadChuck(w, r, filesharingClient)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}
		handleUploadChuck(w, r, filesharingClient)
	})

	http.HandleFunc("/get-storage-info", func(w http.ResponseWriter, r *http.Request) {
		handleGetStorageInfo(w, r, filesharingClient)
	})

	http.HandleFunc("/get-chunk", func(w http.ResponseWriter, r *http.Request) {
		handleGetFileChunk(w, r, filesharingClient)
	})

	http.HandleFunc("/filesharing", func(w http.ResponseWriter, r *http.Request) {
		serveUnifiedPage(w, r)
	})

	http.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		serveUnifiedPage(w, r)
	})

	// Add route for direct file download links
	http.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		// Extract filename from URL path
		filename := r.URL.Path[len("/download/"):]
		if filename == "" {
			http.Error(w, "Filename not provided", http.StatusBadRequest)
			return
		}

		// Redirect to app page with filename parameter
		redirectURL := fmt.Sprintf("/app?filename=%s", filename)
		http.Redirect(w, r, redirectURL, http.StatusFound)
	})

	http.HandleFunc("/streamsaver/mitm.html", func(w http.ResponseWriter, r *http.Request) {
		staticDir := filepath.Join(".", "static")
		filePath := filepath.Join(staticDir, "mitm.html")

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			log.Printf("File not found: %s", filePath)
			http.Error(w, "Page not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFile(w, r, filePath)
	})

	http.HandleFunc("/streamsaver/sw.js", func(w http.ResponseWriter, r *http.Request) {
		staticDir := filepath.Join(".", "static")
		filePath := filepath.Join(staticDir, "sw.js")

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			log.Printf("File not found: %s", filePath)
			http.Error(w, "Service worker not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		http.ServeFile(w, r, filePath)
	})

	log.Println("Gateway HTTP server starting on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
