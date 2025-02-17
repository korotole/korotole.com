package main

import (
	"log"

	web_rdb "website/redis"
	web_rtr "website/router"
)

const (
	ListenAddr = ""
	RedisAddr  = ""
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
