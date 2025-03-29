package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type NewsletterSubscription struct {
	Email      string `json:"email"`
	IsActive   bool   `json:"is_active"`
	SessionID  string `json:"session_id"`
	ModifiedAt string `json:"modified_at,omitempty"` // For responses only
	CreatedAt  string `json:"created_at,omitempty"`  // For responses only
}

func InitNewsletterDB(db *sql.DB) error {
	// Create the Newsletter table if it doesn't exist
	createNewsletterTableSQL := `
		CREATE TABLE IF NOT EXISTS Newsletter (
			id INT AUTO_INCREMENT PRIMARY KEY,
			email VARCHAR(255) NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			session_id VARCHAR(255) NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			modified_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX (email),
			INDEX (session_id)
		)`

	_, err := db.Exec(createNewsletterTableSQL)
	if err != nil {
		return err
	}

	return nil
}

func HandleNewsletterSubscriptions(w http.ResponseWriter, r *http.Request) {
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

		// No need to check if the session exists, since the cookie was processed already
		// and is being written to the database in parallel

		// Check if the subscription already exists
		var exists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM Newsletter WHERE email = ?)", subscription.Email).Scan(&exists)
		if err != nil {
			http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if exists {
			// Update existing subscription
			_, err = db.Exec(
				"UPDATE Newsletter SET is_active = ?, session_id = ? WHERE email = ?",
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
				"INSERT INTO Newsletter (email, is_active, session_id) VALUES (?, ?, ?)",
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
		rows, err := db.Query("SELECT email, is_active, session_id, created_at, modified_at FROM Newsletter")
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

func HandleNewsletterByEmail(w http.ResponseWriter, r *http.Request) {
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
			"SELECT is_active, session_id, created_at, modified_at FROM Newsletter WHERE email = ?",
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
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM Newsletter WHERE email = ?)", email).Scan(&exists)
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
			"UPDATE Newsletter SET is_active = ?, session_id = ? WHERE email = ?",
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
		_, err := db.Exec("DELETE FROM Newsletter WHERE email = ?", email)
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
