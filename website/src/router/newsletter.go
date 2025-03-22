package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// NewsletterSubscription struct (copy from the database service)
type NewsletterSubscription struct {
	Email      string `json:"email"`
	IsActive   bool   `json:"is_active"`
	SessionID  string `json:"session_id"`
	ModifiedAt string `json:"modified_at,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

func RegisterForNewsletter(email string, sessionID string) (int, string) {

	// Create the subscription request
	subscriptionRequest := NewsletterSubscription{
		Email:     email,
		IsActive:  true,
		SessionID: sessionID,
	}

	// Convert the request to JSON
	requestBody, err := json.Marshal(subscriptionRequest)
	if err != nil {
		log.Printf("Error creating JSON request: %v", err)
		return http.StatusInternalServerError, "Server error"
	}

	// Set up the HTTP request to the database service
	dbServiceURL := os.Getenv("DB_SERVICE_URL")
	if dbServiceURL == "" {
		dbServiceURL = "http://database:8082" // Default if not set
	}

	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/newsletters", dbServiceURL),
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		log.Printf("Error creating HTTP request: %v", err)
		return http.StatusInternalServerError, "Server error"
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send the request to the database service
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending request to database service: %v", err)
		return http.StatusInternalServerError, "Error processing subscription"
	}

	defer resp.Body.Close()

	// Check the response
	if resp.StatusCode != http.StatusOK {
		// Read error message from the response body
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Database service returned error: %s (Status: %d)", string(body), resp.StatusCode)
		return http.StatusInternalServerError, "Error processing subscription"
	}

	// Registration successful
	log.Printf("Newsletter registration successful for email: %s", email)

	// Prepare success message for the template
	return http.StatusOK, "Subscription successful"
}
