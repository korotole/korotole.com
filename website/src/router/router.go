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
	http.HandleFunc("/login", SessionControl(loginHandler))
	http.HandleFunc("/donate", SessionControl(donateHandler))
}

func Run(ListenAddr string) {
	log.Println("Starting webserver at ", ListenAddr)
	log.Fatal(http.ListenAndServe(":"+strings.Split(ListenAddr, ":")[1], nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tpl.ExecuteTemplate(w, "index.html", visitors)
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
