package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"

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

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := scheme + "://" + r.Host

	url_final, err := client.ShortURL(r.Context(), &shortener.ShortURLRequest{
		BaseURL:     baseURL,
		OriginalURL: url,
	})
	if err != nil {
		http.Error(w, "Erro ao encurtar URL", http.StatusInternalServerError)
		return
	}
	w.Write([]byte(url_final.ShortURL))
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
		CurrentUrl:  baseURL + "/get?fileName=" + filename,
	})
	if err != nil {
		fmt.Printf("Error uploading file: %v\n", err)
		http.Error(w, "Erro ao fazer upload do ficheiro", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "File uploaded successfully: %s\nFile URL: %s", res.FileName, res.FileURL)
}

func handleGetFile(w http.ResponseWriter, r *http.Request, client filesharing.FileUploadClient) {
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

	res, err := client.GetFile(r.Context(), &filesharing.GetFileRequest{
		FileName: fileName,
	})
	if err != nil {
		http.Error(w, "Erro ao obter ficheiro", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", res.FileName))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(res.FileContent)
}

func serveUnifiedPage(w http.ResponseWriter, r *http.Request) {
	// Get the absolute path to the static file
	staticDir := filepath.Join(".", "static")
	filePath := filepath.Join(staticDir, "filesharing.html")

	http.ServeFile(w, r, filePath)
}

func main() {
	// Setup shortener connection
	shortenerAddr := "shortener-service:50051"
	shortenerConn, err := grpc.NewClient(shortenerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to shortener service: %v", err)
	}
	defer shortenerConn.Close()
	shortenerClient := shortener.NewShortenerClient(shortenerConn)

	// Setup filesharing connection
	filesharingAddr := "filesharing-service:50052"
	filesharingConn, err := grpc.NewClient(filesharingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
		if r.Method == "OPTIONS" {
			handleUploadFile(w, r, filesharingClient)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}
		handleUploadFile(w, r, filesharingClient)
	})

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		handleGetFile(w, r, filesharingClient)
	})

	http.HandleFunc("/filesharing", func(w http.ResponseWriter, r *http.Request) {
		serveUnifiedPage(w, r)
	})

	// Add route for the main page (root can redirect to the unified page)
	http.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		serveUnifiedPage(w, r)
	})

	log.Println("Gateway HTTP server starting on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
