package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const port = "8080"

type Config struct{
	Rabbit *amqp.Connection
}

func main() {
	// try to connect to rabbitmq
	rabbitConn, err := connect()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	defer rabbitConn.Close()

	app := Config{
		Rabbit: rabbitConn,
	}

	log.Printf("Starting broker service on port %s\n", port)

	// Define the server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: app.routes(),
	}

	//serve the application
	err = srv.ListenAndServe()
	if err != nil {
		log.Panic(err)
	}
}

func connect() (connection *amqp.Connection, err error) {
	var counts int64
	backOff := 1 * time.Second

	for {
		c, err := amqp.Dial("amqp://guest:guest@rabbitmq")

		if err != nil {
			fmt.Println("Rabbimq not yet ready")
			counts++
		} else {
			log.Println("Rabbitmq connected!")
			connection = c
			break
		}

		if counts > 5 {
			fmt.Println(err)
			return nil, err
		}

		backOff = time.Duration(math.Pow(float64(counts), 2)) * time.Second
		log.Println("Backing off for " + backOff.String() + "s")
		time.Sleep(backOff)
		continue
	}

	return connection, err
}
