package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Maruqes/KubeFile/shared/proto/filesharing"
	"github.com/Maruqes/KubeFile/shared/proto/shortener"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
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
		http.Error(w, "Erro ao obter chunk do ficheiro "+err.Error(), http.StatusInternalServerError)
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

func handlePublicDownload(w http.ResponseWriter, r *http.Request, client filesharing.FileUploadClient) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	rawName := strings.TrimPrefix(r.URL.Path, "/download/")
	if rawName == "" {
		http.Error(w, "Filename not provided", http.StatusBadRequest)
		return
	}

	fileName, err := url.PathUnescape(rawName)
	if err != nil {
		http.Error(w, "Filename inválido", http.StatusBadRequest)
		return
	}
	fileName = filepath.Base(fileName)
	if fileName == "" || fileName == "." {
		http.Error(w, "Filename inválido", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	flusher, _ := w.(http.Flusher)
	chunkIndex := int32(0)

	for {
		res, err := client.GetChunk(r.Context(), &filesharing.GetChunkRequest{
			FileName:   fileName,
			ChunkIndex: chunkIndex,
		})
		if err != nil {
			errMsg := strings.ToLower(err.Error())
			if chunkIndex == 0 && (strings.Contains(errMsg, "does not exist") || strings.Contains(errMsg, "no such key") || strings.Contains(errMsg, "not provided")) {
				http.Error(w, "Ficheiro não encontrado", http.StatusNotFound)
				return
			}

			st, ok := status.FromError(err)
			if ok {
				switch st.Code() {
				case codes.NotFound:
					http.Error(w, "Ficheiro não encontrado", http.StatusNotFound)
					return
				case codes.DeadlineExceeded:
					http.Error(w, "Pedido expirou", http.StatusGatewayTimeout)
					return
				}
			}

			log.Printf("Erro ao transferir ficheiro %s chunk %d: %v", fileName, chunkIndex, err)
			http.Error(w, "Erro ao transferir ficheiro", http.StatusInternalServerError)
			return
		}

		if len(res.ChunkData) > 0 {
			if _, err := w.Write(res.ChunkData); err != nil {
				log.Printf("Erro ao escrever chunk %d do ficheiro %s: %v", chunkIndex, fileName, err)
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}

		if res.IsLastChunk {
			break
		}

		chunkIndex++
	}
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

func serveLoginPage(w http.ResponseWriter, r *http.Request) {
	staticDir := filepath.Join(".", "static")
	filePath := filepath.Join(staticDir, "login.html")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Login page not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, filePath)
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func parseBoolEnv(key string, defaultVal bool) bool {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return defaultVal
	}
	return parsed
}

func validateCredentials(inputUser, inputPass, expectedUser, expectedPass string) bool {
	userOK := subtle.ConstantTimeCompare([]byte(inputUser), []byte(expectedUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(inputPass), []byte(expectedPass)) == 1
	return userOK && passOK
}

func generateSessionToken(username string, secret []byte, duration time.Duration) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	exp := time.Now().Add(duration).Unix()
	payload := fmt.Sprintf("%s|%d|%s", username, exp, base64.RawURLEncoding.EncodeToString(nonce))

	mac := hmac.New(sha256.New, secret)
	if _, err := mac.Write([]byte(payload)); err != nil {
		return "", err
	}
	signature := mac.Sum(nil)

	token := payload + "." + base64.RawURLEncoding.EncodeToString(signature)
	return token, nil
}

func verifySessionToken(token string, secret []byte) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	payload, sigEncoded := parts[0], parts[1]

	sig, err := base64.RawURLEncoding.DecodeString(sigEncoded)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, secret)
	if _, err := mac.Write([]byte(payload)); err != nil {
		return false
	}
	expectedSig := mac.Sum(nil)
	if subtle.ConstantTimeCompare(expectedSig, sig) != 1 {
		return false
	}

	payloadParts := strings.SplitN(payload, "|", 3)
	if len(payloadParts) != 3 {
		return false
	}

	expiryUnix, err := strconv.ParseInt(payloadParts[1], 10, 64)
	if err != nil {
		return false
	}

	return time.Now().Unix() <= expiryUnix
}

func setAuthCookie(w http.ResponseWriter, cookieName, token string, secure bool, duration time.Duration) {
	c := &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(duration),
	}
	http.SetCookie(w, c)
}

func isAuthenticated(r *http.Request, cookieName string, secret []byte) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	return verifySessionToken(c.Value, secret)
}

func authMiddleware(cookieName string, secret []byte, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isAuthenticated(r, cookieName, secret) {
			next.ServeHTTP(w, r)
			return
		}
		// Redirect unauthenticated users to login
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request, userEnv, passEnv, cookieName string, secret []byte, cookieSecure bool, sessionDuration time.Duration) {
	if r.Method == http.MethodGet {
		serveLoginPage(w, r)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Pedido inválido", http.StatusBadRequest)
		return
	}
	u := r.Form.Get("username")
	p := r.Form.Get("password")

	if !validateCredentials(u, p, userEnv, passEnv) {
		http.Error(w, "Credenciais inválidas", http.StatusUnauthorized)
		return
	}

	token, err := generateSessionToken(u, secret, sessionDuration)
	if err != nil {
		log.Printf("error generating session token: %v", err)
		http.Error(w, "Erro interno ao iniciar sessão", http.StatusInternalServerError)
		return
	}

	secureFlag := cookieSecure || r.TLS != nil
	setAuthCookie(w, cookieName, token, secureFlag, sessionDuration)
	http.Redirect(w, r, "/app", http.StatusFound)
}

func main() {
	maxMsgSize := 31 * 1024 * 1024 // 6MB
	sessionDuration := 24 * time.Hour * 8

	// Setup shortener connection
	shortenerAddr := getEnv("SHORTENER_SERVICE_ADDR", "shortener-service.kubefile.svc.cluster.local:50051")
	shortenerConn, err := grpc.NewClient(shortenerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize), grpc.MaxCallSendMsgSize(maxMsgSize)))
	if err != nil {
		log.Fatalf("Failed to connect to shortener service: %v", err)
	}
	defer shortenerConn.Close()
	shortenerClient := shortener.NewShortenerClient(shortenerConn)

	// Setup filesharing connection
	filesharingAddr := getEnv("FILESHARING_SERVICE_ADDR", "filesharing-service.kubefile.svc.cluster.local:50052")
	filesharingConn, err := grpc.NewClient(filesharingAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize), grpc.MaxCallSendMsgSize(maxMsgSize)))
	if err != nil {
		log.Fatalf("Failed to connect to filesharing service: %v", err)
	}
	defer filesharingConn.Close()
	filesharingClient := filesharing.NewFileUploadClient(filesharingConn)

	// Auth configuration from environment (hard-coded in YAML)
	authUser := getEnv("AUTH_USERNAME", "")
	authPass := getEnv("AUTH_PASSWORD", "")
	authSecret := getEnv("AUTH_SECRET", "")
	if authUser == "" || authPass == "" {
		log.Fatal("AUTH_USERNAME and AUTH_PASSWORD must be provided")
	}
	if len(authSecret) < 32 {
		log.Fatal("AUTH_SECRET must be set (32+ characters recommended)")
	}
	sessionCookieName := getEnv("SESSION_COOKIE_NAME", "kubefile_session")
	cookieSecure := parseBoolEnv("COOKIE_SECURE", false)
	if !cookieSecure {
		log.Println("WARNING: secure cookies are disabled; enable COOKIE_SECURE=true when serving over HTTPS")
	}
	secretBytes := []byte(authSecret)

	// Configure HTTP routes
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app", http.StatusFound)
	})

	http.HandleFunc("/short", authMiddleware(sessionCookieName, secretBytes, func(w http.ResponseWriter, r *http.Request) {
		askForShortURL(w, r, shortenerClient)
	}))

	http.HandleFunc("/geturl", func(w http.ResponseWriter, r *http.Request) {
		getMainUrl(w, r, shortenerClient)
	})

	http.HandleFunc("/upload", authMiddleware(sessionCookieName, secretBytes, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}
		handleUploadFile(w, r, filesharingClient)
	}))

	http.HandleFunc("/upload-chunk", authMiddleware(sessionCookieName, secretBytes, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			handleUploadChuck(w, r, filesharingClient)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}
		handleUploadChuck(w, r, filesharingClient)
	}))

	http.HandleFunc("/get-storage-info", authMiddleware(sessionCookieName, secretBytes, func(w http.ResponseWriter, r *http.Request) {
		handleGetStorageInfo(w, r, filesharingClient)
	}))

	http.HandleFunc("/get-chunk", authMiddleware(sessionCookieName, secretBytes, func(w http.ResponseWriter, r *http.Request) {
		handleGetFileChunk(w, r, filesharingClient)
	}))

	http.HandleFunc("/filesharing", authMiddleware(sessionCookieName, secretBytes, func(w http.ResponseWriter, r *http.Request) {
		serveUnifiedPage(w, r)
	}))

	http.HandleFunc("/app", authMiddleware(sessionCookieName, secretBytes, func(w http.ResponseWriter, r *http.Request) {
		serveUnifiedPage(w, r)
	}))

	// Public route for direct file downloads
	http.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		handlePublicDownload(w, r, filesharingClient)
	})

	http.HandleFunc("/streamsaver/mitm.html", authMiddleware(sessionCookieName, secretBytes, func(w http.ResponseWriter, r *http.Request) {
		staticDir := filepath.Join(".", "static")
		filePath := filepath.Join(staticDir, "mitm.html")

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			log.Printf("File not found: %s", filePath)
			http.Error(w, "Page not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFile(w, r, filePath)
	}))

	http.HandleFunc("/streamsaver/sw.js", authMiddleware(sessionCookieName, secretBytes, func(w http.ResponseWriter, r *http.Request) {
		staticDir := filepath.Join(".", "static")
		filePath := filepath.Join(staticDir, "sw.js")

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			log.Printf("File not found: %s", filePath)
			http.Error(w, "Service worker not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		http.ServeFile(w, r, filePath)
	}))

	// Login route (unprotected)
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		handleLogin(w, r, authUser, authPass, sessionCookieName, secretBytes, cookieSecure, sessionDuration)
	})

	log.Println("Gateway HTTP server starting on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
