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
		// log.Println("cookie:", cookie, "err:", err)
		if err != nil {
			clientIP, ssnId, tmStmp := establishSession(&w, r)
			// Notify the Telegram bot microservice about the new connection
			go notifyTelegramBot(clientIP)
			// store the data via database microservice
			go updateDatabase(ssnId, clientIP, tmStmp)
		}
		HandlerFunc.ServeHTTP(w, r)
	}
}

func establishSession(w *http.ResponseWriter, r *http.Request) (string, string, string) {
	// take dockerization & cloudflare deployment into account
	clientIP := r.Header.Get("CF-Connecting-IP")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Forwarded-For")
	}
	if clientIP == "" {
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

	http.SetCookie(*w, cookie)
	log.Println("Website accessed: ", clientIP)
	log.Println("Session created: ", hash)

	visitors, err = rdb.Client.Incr(web_rdb.Ctx, visitorCountKey).Result()
	if err != nil {
		log.Println("Error: ", err)
	}

	return clientIP, hash, timestamp
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

func updateDatabase(ssnId string, IP string, tmStmp string) {

}
