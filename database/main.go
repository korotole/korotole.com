// db-service/main.go - The main entry point for the database microservice

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var (
	// Database connection
	db *sql.DB
)

type SessionRequest struct {
	SessionID string `json:"session_id"`
	IPAddress string `json:"ip_address"`
	Timestamp string `json:"timestamp"`
	UserAgent string `json:"user_agent"`
	Action    string `json:"action"`
}

type NewsletterSubscription struct {
	Email      string `json:"email"`
	IsActive   bool   `json:"is_active"`
	SessionID  string `json:"session_id"`
	ModifiedAt string `json:"modified_at,omitempty"` // For responses only
	CreatedAt  string `json:"created_at,omitempty"`  // For responses only
}

func main() {
	// Initialize database connection
	if err := initDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Register HTTP handlers
	http.HandleFunc("/sessions", handleSessions)
	http.HandleFunc("/sessions/", handleSessionByID)
	http.HandleFunc("/newsletters", handleNewsletterSubscriptions)
	http.HandleFunc("/newsletters/", handleNewsletterByEmail)
	http.HandleFunc("/health", healthCheck)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	log.Printf("Database microservice starting on port %s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initDB() error {
	// Use variables from env.go
	dbUser := os.Getenv("MYSQL_USERNAME")
	dbPass := os.Getenv("MYSQL_PASSWORD")
	dbHost := os.Getenv("MYSQL_HOST")
	dbPort := os.Getenv("MYSQL_PORT")
	dbName := os.Getenv("MYSQL_DATABASE")

	// Create the connection string
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		dbUser, dbPass, dbHost, dbPort, dbName)
	log.Printf("Connecting to database at %s\n", dsn)

	// Connect to the database
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		return err
	}

	// Test the connection
	err = db.Ping()
	if err != nil {
		return err
	}

	// Set connection pool parameters
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Create the sessions table if it doesn't exist
	createSessionsTableSQL := `
    CREATE TABLE IF NOT EXISTS sessions (
        id INT AUTO_INCREMENT PRIMARY KEY,
        session_id VARCHAR(255) NOT NULL,
        ip_address VARCHAR(50) NOT NULL,
        timestamp BIGINT NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP,
        user_agent TEXT,
        INDEX (session_id),
        INDEX (ip_address)
    )`

	_, err = db.Exec(createSessionsTableSQL)
	if err != nil {
		return err
	}

	// Create the newsletter_subscriptions table if it doesn't exist
	createNewsletterTableSQL := `
    CREATE TABLE IF NOT EXISTS newsletter_subscriptions (
        id INT AUTO_INCREMENT PRIMARY KEY,
        email VARCHAR(255) NOT NULL,
        is_active BOOLEAN NOT NULL DEFAULT TRUE,
        session_id VARCHAR(255) NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        modified_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        INDEX (email),
        INDEX (session_id)
    )`

	_, err = db.Exec(createNewsletterTableSQL)
	if err != nil {
		return err
	}

	log.Println("Database connection established and tables created")
	return nil
}

func handleSessions(w http.ResponseWriter, r *http.Request) {
	// Only handle POST requests for this endpoint
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req SessionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	// Validate the session ID
	if req.SessionID == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	// Handle different actions
	switch req.Action {
	case "create":
		if err := createSession(req); err != nil {
			http.Error(w, "Failed to create session: "+err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func handleSessionByID(w http.ResponseWriter, r *http.Request) {
	// Only handle GET requests for this endpoint
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID from the URL path
	// URL format: /sessions/{id}
	path := r.URL.Path
	parts := strings.Split(path, "/")

	// Ensure we have enough parts in the path
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	sessionID := parts[2]
	if sessionID == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	// Query the database
	var (
		id           int
		ip           string
		timestamp    int64
		createdAt    time.Time
		lastAccessed time.Time
		userAgent    string
	)

	err := db.QueryRow(
		"SELECT id, ip_address, timestamp, created_at, last_accessed, user_agent FROM sessions WHERE session_id = ?",
		sessionID,
	).Scan(&id, &ip, &timestamp, &createdAt, &lastAccessed, &userAgent)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Session not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Return session data
	sessionData := map[string]interface{}{
		"id":            id,
		"session_id":    sessionID,
		"ip_address":    ip,
		"timestamp":     timestamp,
		"created_at":    createdAt.Format(time.RFC3339),
		"last_accessed": lastAccessed.Format(time.RFC3339),
		"user_agent":    userAgent,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessionData)
}

func handleNewsletterSubscriptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		// Create or update a newsletter subscription
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var subscription NewsletterSubscription
		if err := json.Unmarshal(body, &subscription); err != nil {
			http.Error(w, "Invalid JSON format", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if subscription.Email == "" {
			http.Error(w, "Missing email", http.StatusBadRequest)
			return
		}

		if subscription.SessionID == "" {
			http.Error(w, "Missing session ID", http.StatusBadRequest)
			return
		}

		// Check if the session exists
		var sessionExists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM sessions WHERE session_id = ?)", subscription.SessionID).Scan(&sessionExists)
		if err != nil {
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !sessionExists {
			http.Error(w, "Invalid session ID", http.StatusBadRequest)
			return
		}

		// Check if the subscription already exists
		var exists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM newsletter_subscriptions WHERE email = ?)", subscription.Email).Scan(&exists)
		if err != nil {
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if exists {
			// Update existing subscription
			_, err = db.Exec(
				"UPDATE newsletter_subscriptions SET is_active = ?, session_id = ? WHERE email = ?",
				subscription.IsActive, subscription.SessionID, subscription.Email,
			)
			if err != nil {
				http.Error(w, "Failed to update subscription: "+err.Error(), http.StatusInternalServerError)
				return
			}
			log.Printf("Updated newsletter subscription for email: %s\n", subscription.Email)
		} else {
			// Create new subscription
			_, err = db.Exec(
				"INSERT INTO newsletter_subscriptions (email, is_active, session_id) VALUES (?, ?, ?)",
				subscription.Email, subscription.IsActive, subscription.SessionID,
			)
			if err != nil {
				http.Error(w, "Failed to create subscription: "+err.Error(), http.StatusInternalServerError)
				return
			}
			log.Printf("Created newsletter subscription for email: %s\n", subscription.Email)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})

	case http.MethodGet:
		// List all newsletter subscriptions
		rows, err := db.Query("SELECT email, is_active, session_id, created_at, modified_at FROM newsletter_subscriptions")
		if err != nil {
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var subscriptions []NewsletterSubscription
		for rows.Next() {
			var (
				email      string
				isActive   bool
				sessionID  string
				createdAt  time.Time
				modifiedAt time.Time
			)

			if err := rows.Scan(&email, &isActive, &sessionID, &createdAt, &modifiedAt); err != nil {
				http.Error(w, "Error scanning database results: "+err.Error(), http.StatusInternalServerError)
				return
			}

			subscriptions = append(subscriptions, NewsletterSubscription{
				Email:      email,
				IsActive:   isActive,
				SessionID:  sessionID,
				CreatedAt:  createdAt.Format(time.RFC3339),
				ModifiedAt: modifiedAt.Format(time.RFC3339),
			})
		}

		if err := rows.Err(); err != nil {
			http.Error(w, "Error iterating over results: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(subscriptions)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleNewsletterByEmail(w http.ResponseWriter, r *http.Request) {
	// Extract email from the URL path
	// URL format: /newsletters/{email}
	path := r.URL.Path
	parts := strings.Split(path, "/")

	// Ensure we have enough parts in the path
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	email := parts[2]
	if email == "" {
		http.Error(w, "Missing email", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Get a specific newsletter subscription
		var (
			isActive   bool
			sessionID  string
			createdAt  time.Time
			modifiedAt time.Time
		)

		err := db.QueryRow(
			"SELECT is_active, session_id, created_at, modified_at FROM newsletter_subscriptions WHERE email = ?",
			email,
		).Scan(&isActive, &sessionID, &createdAt, &modifiedAt)

		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Subscription not found", http.StatusNotFound)
			} else {
				http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}

		subscription := NewsletterSubscription{
			Email:      email,
			IsActive:   isActive,
			SessionID:  sessionID,
			CreatedAt:  createdAt.Format(time.RFC3339),
			ModifiedAt: modifiedAt.Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(subscription)

	case http.MethodPut:
		// Update a specific newsletter subscription
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var subscription NewsletterSubscription
		if err := json.Unmarshal(body, &subscription); err != nil {
			http.Error(w, "Invalid JSON format", http.StatusBadRequest)
			return
		}

		// Override the email in the URL with the one in the path
		subscription.Email = email

		// Validate session ID
		if subscription.SessionID == "" {
			http.Error(w, "Missing session ID", http.StatusBadRequest)
			return
		}

		// Check if the subscription exists
		var exists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM newsletter_subscriptions WHERE email = ?)", email).Scan(&exists)
		if err != nil {
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !exists {
			http.Error(w, "Subscription not found", http.StatusNotFound)
			return
		}

		// Update the subscription
		_, err = db.Exec(
			"UPDATE newsletter_subscriptions SET is_active = ?, session_id = ? WHERE email = ?",
			subscription.IsActive, subscription.SessionID, email,
		)
		if err != nil {
			http.Error(w, "Failed to update subscription: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Updated newsletter subscription for email: %s\n", email)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})

	case http.MethodDelete:
		// Delete a specific newsletter subscription
		_, err := db.Exec("DELETE FROM newsletter_subscriptions WHERE email = ?", email)
		if err != nil {
			http.Error(w, "Failed to delete subscription: "+err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Deleted newsletter subscription for email: %s\n", email)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	// Only handle GET requests for health check
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check database connection
	err := db.Ping()
	if err != nil {
		http.Error(w, "Database not reachable", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func createSession(req SessionRequest) error {
	// Parse timestamp to int64
	timestampInt, err := strconv.ParseInt(req.Timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("error parsing timestamp: %v", err)
	}

	// Check if the session already exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sessions WHERE session_id = ?", req.SessionID).Scan(&count)
	if err != nil {
		return fmt.Errorf("error checking for existing session: %v", err)
	}

	if count > 0 {
		// Update existing session
		_, err = db.Exec(
			"UPDATE sessions SET last_accessed = CURRENT_TIMESTAMP WHERE session_id = ?",
			req.SessionID,
		)
		if err != nil {
			return fmt.Errorf("error updating session: %v", err)
		}
	} else {
		// Insert new session
		_, err = db.Exec(
			"INSERT INTO sessions (session_id, ip_address, timestamp, user_agent) VALUES (?, ?, ?, ?)",
			req.SessionID, req.IPAddress, timestampInt, req.UserAgent,
		)
		if err != nil {
			return fmt.Errorf("error inserting session: %v", err)
		}
		log.Printf("Session stored in database: %s\n", req.SessionID)
	}

	return nil
}
