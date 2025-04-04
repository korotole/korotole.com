package router

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	web_rdb "website/redis"
	web_utl "website/utils"
)

var (
	err error
	rdb *web_rdb.Redis
	tpl *template.Template

	// Visitor/Redis
	visitorCountKey string = "visitor-count"
	visitors        int64  = 0 // TODO: get actual number from beginning?????
)

func InitRouter(database *web_rdb.Redis) {
	rdb = database
	tpl = template.Must(template.ParseGlob(filepath.Join(web_utl.GetBaseDir(), "static/templates/*.html")))

	// initialize visitor count at the very beginning
	visitors, err = rdb.Client.Get(web_rdb.Ctx, visitorCountKey).Int64()
	if err != nil {
		log.Println("Error while initializing visitors count: ", err)
	}

	// Properly serve all static files (CSS, JS, images, icons)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(web_utl.GetBaseDir(), "static")))))
	// Serve special files (PDFs, images, presentations)
	http.Handle("/files/etc/", http.StripPrefix("/files/etc/", http.FileServer(http.Dir(filepath.Join(web_utl.GetBaseDir(), "files/etc")))))

	// Page handlers
	http.HandleFunc("/", SessionControl(indexHandler))
	http.HandleFunc("/cv", SessionControl(cvHandler))
	http.HandleFunc("/books", SessionControl(booksHandler))
	http.HandleFunc("/login", SessionControl(loginHandler))
	http.HandleFunc("/donate", SessionControl(donateHandler))
	http.HandleFunc("/newsletter-register", SessionControl(newsletterRegisterHandler))
}

func Run(ListenAddr string) {
	log.Println("Starting webserver at ", ListenAddr)
	log.Fatal(http.ListenAndServe(":"+strings.Split(ListenAddr, ":")[1], nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tpl.ExecuteTemplate(w, "index.html", visitors)
}

func booksHandler(w http.ResponseWriter, r *http.Request) {
	tpl.ExecuteTemplate(w, "books.html", visitors)
}

func donateHandler(w http.ResponseWriter, r *http.Request) {
	tpl.ExecuteTemplate(w, "donate.html", visitors)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
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

func newsletterRegisterHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Newsletter registration attempt detected")

	if r.Method != "POST" {
		log.Println("Invalid request method for newsletter registration")
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(1 << 10); err != nil {
		log.Printf("Error parsing form: %v", err)
		http.Error(w, "Error processing form submission", http.StatusBadRequest)
		return
	}

	// Extract the email from the form
	email := r.FormValue("email")
	if email == "" {
		log.Println("Missing email in newsletter registration form")
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	cookie, err := r.Cookie("session-id")
	if err != nil {
		log.Printf("Error retrieving session ID from cookie: %v", err)
		http.Error(w, "Session ID not found", http.StatusBadRequest)
		return
	}

	sessionID := cookie.Value

	var status, message = RegisterForNewsletter(email, sessionID)
	if status != http.StatusOK {
		log.Printf("Error registering for newsletter: %s", message)
		http.Error(w, message, status)
		return
	}

	// Explicit success response
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Successfully subscribed!"))
}
