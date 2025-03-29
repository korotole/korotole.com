// db-service/main.go - The main entry point for the database microservice

package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var (
	// Database connection
	db *sql.DB
)

func main() {
	// Initialize database connection
	if err := initDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Register HTTP handlers
	http.HandleFunc("/sessions", HandleSessions)
	http.HandleFunc("/sessions/", HandleSessionByID)
	http.HandleFunc("/newsletters", HandleNewsletterSubscriptions)
	http.HandleFunc("/newsletters/", HandleNewsletterByEmail)
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

	err = InitSessionDB(db)
	if err != nil {
		return fmt.Errorf("failed to initialize session database: %v", err)
	}
	err = InitNewsletterDB(db)
	if err != nil {
		return fmt.Errorf("failed to initialize newsletter database: %v", err)
	}

	log.Println("Database connection established and tables created")
	return nil
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
