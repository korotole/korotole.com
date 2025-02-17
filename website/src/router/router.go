package router

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	err    error
	rdb    *web_rdb.Redis
	tpl    *template.Template
	ssnCfg = SessionConfig{
		Name:     "session-id",
		HttpOnly: true,
		MaxAge:   15, // 1/2 hour
	}

	// visitor/Redis
	visitorCountKey string = "visitor-count"
	visitors        int64  = 0
)

func InitRouter(database *web_rdb.Redis) {
	rdb = database
	tpl = template.Must(template.ParseGlob(filepath.Join(web_utl.GetBaseDir(), "static/templates/*.html")))

	// Properly serve all static files (CSS, JS, images, icons)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(web_utl.GetBaseDir(), "static")))))
	// Serve special files (PDFs, images, presentations)
	http.Handle("/files/etc/", http.StripPrefix("/files/etc/", http.FileServer(http.Dir(filepath.Join(web_utl.GetBaseDir(), "files/etc")))))

	// Page handlers
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/cv", cvHandler)
	http.HandleFunc("/login", loginHandler)
}

func Run(ListenAddr string) {
	log.Println("Starting webserver at ", ListenAddr)
	log.Fatal(http.ListenAndServe(":"+strings.Split(ListenAddr, ":")[1], nil))
}

func establishSession(w *http.ResponseWriter, r *http.Request) {
	// DUMMY HASH GEN
	hash := string(web_utl.GetSHA256(r.RemoteAddr + strconv.FormatInt(web_utl.GetTimestamp(), 10)))
	cookie := &http.Cookie{
		Name:     ssnCfg.Name,
		Value:    hash,
		HttpOnly: ssnCfg.HttpOnly,
		MaxAge:   ssnCfg.MaxAge,
	}

	http.SetCookie(*w, cookie)
	log.Println("Website accessed: ", r.RemoteAddr)
	log.Println("Session created: ", hash)

	visitors, err = rdb.Client.Incr(web_rdb.Ctx, visitorCountKey).Result()
	if err != nil {
		log.Println("Error: ", err)
	}
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
	resp, err := http.Post("http://bot:8081/notify-ip", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("Error sending IP to Telegram bot: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Telegram bot microservice returned an error: %v\n", resp.Status)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	_, err := r.Cookie("session-id")
	// log.Println("cookie:", cookie, "err:", err)
	if err != nil {
		establishSession(&w, r)
		// Notify the Telegram bot microservice about the new connection
		notifyTelegramBot(r.RemoteAddr)
	}
	tpl.ExecuteTemplate(w, "index.html", visitors)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	// tpl.ExecuteTemplate(w, "about.html", nil)
	log.Println("Login attempt detected!")
	tpl.ExecuteTemplate(w, "index.html", visitors)
}

func cvHandler(w http.ResponseWriter, r *http.Request) {
	pdfPath := web_utl.GetBaseDir() + "/files/cv.pdf"

	w.Header().Set("Content-Type", "application/pdf")

	file, err := os.Open(pdfPath)
	if err != nil {
		http.Error(w, "File not found:"+pdfPath, http.StatusNotFound)
		return
	}
	defer file.Close()

	http.ServeFile(w, r, pdfPath)
}
