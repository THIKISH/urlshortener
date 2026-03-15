package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

const storageFile = "urls.json"

// We use a RWMutex to ensure that file operations are safe when multiple users access the app concurrently.
var storeMutex sync.RWMutex

func loadURLs() map[string]string {
	store := make(map[string]string)
	data, err := os.ReadFile(storageFile)
	if err == nil {
		json.Unmarshal(data, &store)
	}
	return store
}

func saveURLs(store map[string]string) error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(storageFile, data, 0644)
}

// generateShortCode creates a random 6-character string to be used as the short URL code.
func generateShortCode() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	
	// Read random bytes
	rand.Read(b)
	
	// Map the random bytes to characters in our charset
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

// isValidURL checks if the provided string is a valid URL.
func isValidURL(u string) bool {
	parsedURL, err := url.ParseRequestURI(u)
	if err != nil {
		return false
	}
	// Basic check to ensure it has a scheme (like http/https) and a host.
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return false
	}
	// We only want to accept standard web links.
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false
	}
	return true
}

func main() {
	// 1. GET / -> show the HTML page
	// We handle this along with code redirection because they share the root path "/"
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// If the path is exactly "/", user is visiting the home page. Serve index.html.
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "index.html")
			return
		}

		// 3. GET /{code} -> redirect to the original URL
		// If the path is not "/", we treat it as a short code (e.g., /abc123)
		code := strings.TrimPrefix(r.URL.Path, "/")
		
		// Use RLock (Read Lock) because we are reading from the file
		storeMutex.RLock()
		store := loadURLs()
		originalURL, exists := store[code]
		storeMutex.RUnlock()

		if !exists {
			// If the short code is not in our map, return a 404 Not Found error
			http.Error(w, "Short URL not found", http.StatusNotFound)
			return
		}

		// If found, redirect the user to the original long URL
		http.Redirect(w, r, originalURL, http.StatusFound)
	})

	// 2. POST /shorten -> generate the short URL
	http.HandleFunc("/shorten", func(w http.ResponseWriter, r *http.Request) {
		// Only allow POST requests for this route
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the form data sent from the HTML page
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		// Retrieve the "url" value from the form
		longURL := r.FormValue("url")
		longURL = strings.TrimSpace(longURL)

		// Validate the URL format
		if !isValidURL(longURL) {
			http.Error(w, "Invalid URL provided. Must include http:// or https://", http.StatusBadRequest)
			return
		}

		// Lock for writing to safely add the new URL to the file
		storeMutex.Lock() 
		store := loadURLs()
		code := generateShortCode()
		
		// Handle potential map collision: if code somehow already exists, generate a new one
		for {
			if _, exists := store[code]; !exists {
				break
			}
			code = generateShortCode()
		}
		
		// Save the mapping
		store[code] = longURL
		saveURLs(store)
		storeMutex.Unlock() // Unlock after writing

		// Create the full short URL
		host := r.Host
		shortURL := "https://" + host + "/" + code

		// Return the short URL to the frontend as a JSON response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"shortUrl": shortURL,
		})
	})

	// Start the server
	fmt.Println("Server is running on http://localhost:8080")
	fmt.Println("Press Ctrl+C to stop")
	
	// ListenAndServe blocks indefinitely and listens for incoming requests on port 8080
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
