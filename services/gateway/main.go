package main

import (
	"fmt"
	"log"
	"net/http"

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

func main() {
	shortenerAddr := "shortener-service:50051"

	conn, err := grpc.NewClient(shortenerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Não consegui conectar: %v", err)
	}
	defer conn.Close()

	client := shortener.NewShortenerClient(conn)

	// Configurar HTTP server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Gateway service is running"))
	})

	http.HandleFunc("/short", func(w http.ResponseWriter, r *http.Request) {
		askForShortURL(w, r, client)
	})

	http.HandleFunc("/geturl", func(w http.ResponseWriter, r *http.Request) {
		getMainUrl(w, r, client)
	})

	log.Println("Gateway HTTP rodando na porta 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
