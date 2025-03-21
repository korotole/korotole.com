package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	web_rdb "website/redis"
	web_rtr "website/router"
)

var (
	ListenAddr = os.Getenv("WS_LISTEN_ADDR")
	RedisAddr  = os.Getenv("WS_REDIS_ADDR")
)

var microservices = []string{
	"http://database:8082/health", // DB-service
	"http://bot:8081/health",      // Telegram bot microservice
}

// CheckHealth pings a microservice health endpoint
func CheckHealth(url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("[ERROR] Unable to reach %s: %v\n", url, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[ERROR] Service %s returned status %d: %s\n", url, resp.StatusCode, string(body))
		return false
	}

	log.Printf("[INFO] Service %s is healthy\n", url)
	return true
}

// StartHealthMonitor continuously monitors the health of all microservices
func StartHealthMonitor(interval time.Duration) {
	go func() {
		for {
			for _, url := range microservices {
				isHealthy := CheckHealth(url)
				if !isHealthy {
					log.Printf("[ALERT] Service %s is unhealthy!\n", url)
					// Optional: Send alert to Telegram
					// notifyAdmin(fmt.Sprintf("Service %s is unhealthy!", url))
				}
			}
			time.Sleep(interval)
		}
	}()
}

func main() {
	log.Println("Numer of CPUs: ", runtime.NumCPU()/2)
	runtime.GOMAXPROCS(runtime.NumCPU() / 2)

	db, err := web_rdb.InitRedis(RedisAddr)
	if err != nil {
		log.Fatalf("Failed to connect to redis: %s", err.Error())
	} else {
		log.Println("Redis initialized at ", RedisAddr)
	}

	web_rtr.InitRouter(db)

	StartHealthMonitor(30 * time.Second)

	web_rtr.Run(ListenAddr)
}
