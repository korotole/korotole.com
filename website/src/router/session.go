package router

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	web_rdb "website/redis"
	web_utl "website/utils"
)

// Cookie "session" config
type SessionConfig struct {
	Name     string
	HttpOnly bool
	MaxAge   int
}

var (
	// Parse the session expiration time from environment variable
	ssnCfg = SessionConfig{
		Name:     "session-id",
		HttpOnly: true,
		MaxAge: func() int {
			expireStr := os.Getenv("WS_SSN_EXPIRE")
			expireInt, err := strconv.ParseInt(expireStr, 10, 64)
			if err != nil {
				// Default to 2700 seconds (45 minutes) if there's an error
				return 2700
			}
			return int(expireInt)
		}(),
	}
	// TG bot
	botAddr = "http://bot" + os.Getenv("TG_LISTEN_ADDR") + "/notify-ip"
)

func SessionControl(HandlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := r.Cookie("session-id")

		// new visitor
		if err != nil {
			clientIP, ssnId, tmStmp, usrAgnt := establishSession(&w, r)
			// Notify the Telegram bot microservice about the new connection
			go notifyTelegramBot(clientIP)
			// store the data via database microservice
			go updateDatabase(ssnId, clientIP, tmStmp, usrAgnt)
		}
		HandlerFunc.ServeHTTP(w, r)
	}
}

func establishSession(w *http.ResponseWriter, r *http.Request) (string, string, string, string) {
	// take dockerization & cloudflare deployment into account
	userAgent := r.Header.Get("User-Agent")
	clientIP := r.Header.Get("CF-Connecting-IP")
	if clientIP == "" {
		log.Println("Failed to get CF-Connecting-IP header")
		clientIP = r.Header.Get("X-Forwarded-For")
	}
	if clientIP == "" {
		log.Println("Failed to get X-Forwarded-For header")
		clientIP = r.RemoteAddr
	}

	// DUMMY HASH GEN
	timestamp := strconv.FormatInt(web_utl.GetTimestamp(), 10)
	hash := string(web_utl.GetSHA256(clientIP + timestamp)) // session-id
	cookie := &http.Cookie{
		Name:     ssnCfg.Name,
		Value:    hash,
		HttpOnly: ssnCfg.HttpOnly,
		MaxAge:   ssnCfg.MaxAge,
	}

	// set the cookie for client
	http.SetCookie(*w, cookie)
	// set the cookie for current request
	(*r).AddCookie(cookie)

	log.Println("Website accessed: ", clientIP)
	log.Println("Session created: ", hash)
	log.Println("Timestamp: ", timestamp)
	log.Println("User-Agent: ", userAgent)

	visitors, err = rdb.Client.Incr(web_rdb.Ctx, visitorCountKey).Result()
	if err != nil {
		log.Println("Error: ", err)
	}

	return clientIP, hash, timestamp, userAgent
}

func notifyTelegramBot(ipAddress string) {
	// Create the request payload
	requestData := map[string]string{
		"ip": ipAddress,
	}
	payload, err := json.Marshal(requestData)
	if err != nil {
		log.Printf("Error marshaling IP address: %v\n", err)
		return
	}

	// Send a POST request to the Telegram bot microservice
	resp, err := http.Post(botAddr, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("Error sending IP to Telegram bot: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Telegram bot microservice returned an error: %v\n", resp.Status)
	}
}

func updateDatabase(ssnId string, IP string, tmStmp string, usrAgnt string) {
	// Prepare the request payload for the db-service
	requestData := map[string]string{
		"session_id": ssnId,
		"ip_address": IP,
		"timestamp":  tmStmp,
		"user_agent": usrAgnt, // You can replace this with the actual User-Agent if needed
		"action":     "create",
	}

	payload, err := json.Marshal(requestData)
	if err != nil {
		log.Printf("Error marshaling request data for database: %v\n", err)
		return
	}

	// Define the URL of the db-service
	var dbServiceURL = "http://database:" + os.Getenv("DATABASE_PORT") + "/sessions"

	// Send a POST request to the db-service
	resp, err := http.Post(dbServiceURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("Error sending data to database: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("database returned an error: %v\n", resp.Status)
	} else {
		log.Println("Session data successfully stored")
	}
}
