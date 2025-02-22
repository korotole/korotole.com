package main

import (
	"log"
	"os"

	web_rdb "website/redis"
	web_rtr "website/router"
)

var (
	ListenAddr = os.Getenv("WS_LISTEN_ADDR")
	RedisAddr  = os.Getenv("WS_REDIS_ADDR")
)

func main() {

	db, err := web_rdb.InitRedis(RedisAddr)
	if err != nil {
		log.Fatalf("Failed to connect to redis: %s", err.Error())
	} else {
		log.Println("Redis initialized at ", RedisAddr)
	}

	web_rtr.InitRouter(db)
	web_rtr.Run(ListenAddr)
}
