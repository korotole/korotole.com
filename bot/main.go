package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type IPInfo struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Loc      string `json:"loc"`
	Org      string `json:"org"`
	Postal   string `json:"postal"`
	Timezone string `json:"timezone"`
}

func getIPInfo(ip string) (*IPInfo, error) {
	// Replace with a valid API key if needed
	url := fmt.Sprintf("http://ipinfo.io/%s/json", ip)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ipInfo IPInfo
	err = json.Unmarshal(body, &ipInfo)
	if err != nil {
		return nil, err
	}

	return &ipInfo, nil
}

var (
	telegramBot    *tgbotapi.BotAPI
	telegramChatID int64
)

func init() {
	// Initialize the Telegram bot
	var err error
	telegramBot, err = tgbotapi.NewBotAPI(os.Getenv("TG_API_KEY"))
	if err != nil {
		log.Fatal("Failed to initialize Telegram bot: ", err)
	}

	// Set the Telegram chat ID (this can be found through bot interaction)
	telegramChatID = 448580548
}

func notifyAdmin(message string) {
	// Telegram Notification
	msg := tgbotapi.NewMessage(telegramChatID, message)
	msg.ParseMode = tgbotapi.ModeMarkdown // support for bold / italic text
	_, err := telegramBot.Send(msg)
	if err != nil {
		log.Printf("Failed to send Telegram message: %v\n", err)
	}
}

type IpRequest struct {
	IP string `json:"ip"`
}

func formatIPInfo(ipInfo *IPInfo) string {
	return fmt.Sprintf("*IP:* `%s`\n*Hostname:* `%s`\n*City:* `%s`\n*Region:* `%s`\n*Country:* `%s`\n*Location (Lat/Lon):* `%s`\n*Organization:* `%s`\n*Postal Code:* `%s`\n*Timezone:* `%s`\n",
		ipInfo.IP, ipInfo.Hostname, ipInfo.City, ipInfo.Region, ipInfo.Country, ipInfo.Loc, ipInfo.Org, ipInfo.Postal, ipInfo.Timezone)

}

func ipConnectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Parse the incoming JSON request
	var requestData IpRequest
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		http.Error(w, "Error parsing IP address", http.StatusBadRequest)
		return
	}

	// Notify the admin via Telegram
	ipAddress := requestData.IP
	if strings.Contains(ipAddress, ":") {
		ipAddress = ipAddress[:strings.LastIndex(ipAddress, ":")]
	}

	var msg string
	info, err := getIPInfo(ipAddress)
	if err != nil {
		msg = fmt.Sprintf("Error while getting visitor's IP info: %s", err.Error())
	} else {
		msg = formatIPInfo(info)
	}

	log.Println(msg)
	notifyAdmin(msg)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Notification sent successfully"))
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	// Prepare the health status response
	status := "ok"
	statusCode := http.StatusOK
	botStatus := "initialized"

	// Check if the Telegram bot is properly initialized
	if telegramBot == nil {
		status = "error"
		botStatus = "not initialized"
		statusCode = http.StatusInternalServerError
	}

	// Check required environment variables
	tgApiKey := os.Getenv("TG_API_KEY")
	tgListenAddr := os.Getenv("TG_LISTEN_ADDR")
	if tgApiKey == "" || tgListenAddr == "" {
		status = "error"
		statusCode = http.StatusInternalServerError
	}

	// Check connection to Telegram:
	// Send a "getMe" request to Telegram API
	_, err := telegramBot.GetMe()
	if err != nil {
		status = "error"
		botStatus = "unreachable"
		statusCode = http.StatusInternalServerError
	}

	// Build the JSON response
	healthResponse := map[string]interface{}{
		"status":           status,
		"telegram_bot":     botStatus,
		"telegram_api_key": tgApiKey != "",
		"listen_address":   tgListenAddr,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(healthResponse)
}

func main() {
	http.HandleFunc("/notify-ip", ipConnectionHandler)
	http.HandleFunc("/health", healthCheck)

	// Start the server
	log.Println("Telegram bot microservice running at" + os.Getenv("TG_LISTEN_ADDR"))
	log.Fatal(http.ListenAndServe(os.Getenv("TG_LISTEN_ADDR"), nil))
}
