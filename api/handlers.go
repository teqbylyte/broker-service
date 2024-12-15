package main

import (
	"broker/event"
	"broker/logs"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/rpc"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RequestPayload struct {
	Action string `json:"action"`
	Auth AuthPayload `json:"auth,omitempty"`
	Log LogPayload `json:"log,omitempty"`
}

type AuthPayload struct {
	Email string `json:"email"`
	Password string `json:"password"`
}

type LogPayload struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type MailPayload struct {
	To string `json:"to"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

func (app *Config) Broker(w http.ResponseWriter, r *http.Request) {
	payload := jsonResponse{
		Status:  true,
		Message: "Hit the broker",
	}

	app.writeJSON(w, http.StatusOK, payload)
}

func (app *Config) HandleSubmission(w http.ResponseWriter, r *http.Request) {
	var requestPayload RequestPayload

	err := app.readJSON(w, r, &requestPayload)
	if err != nil {
		log.Println(err)
		app.errorJSON(w, err)
		return
	}

	switch requestPayload.Action {
	case "auth":
		app.Authenticate(w, requestPayload.Auth)
	case "log":
		// app.LogItem(w, requestPayload.Log)
		// app.logEventViaRabbit(w, requestPayload.Log)
		// app.logItemViaRPC(w, requestPayload.Log)
		app.LogViaGRPC(w, requestPayload.Log)
	default:
		app.errorJSON(w, errors.New("unknown action"))
	}
}

func (app *Config) Authenticate(w http.ResponseWriter, a AuthPayload) {
	// create json to send to the auth microservice
	jsonData, _ := json.MarshalIndent(a, "", "\t")

	// call auth service
	request, err := http.NewRequest("POST", "http://auth-service/authenticate", bytes.NewBuffer(jsonData))
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	client := &http.Client{}
	res, err := client.Do(request)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	defer res.Body.Close()

	// check status code returned
	if res.StatusCode == http.StatusUnauthorized {
		app.errorJSON(w, errors.New("invalid credentials"))
		return
	} else if res.StatusCode != http.StatusAccepted {
		app.errorJSON(w, errors.New("error calling auth service"))
		return
	}

	// create a variable
	var serviceJson jsonResponse
	err = json.NewDecoder(res.Body).Decode(&serviceJson)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	if !serviceJson.Status {
		app.errorJSON(w, errors.New(serviceJson.Message), http.StatusUnauthorized)
		return
	} 
	
	log.Println("Successful auth")

	// Send mail
	go app.sendMail(MailPayload{
		To: a.Email,
		Subject: "Sign In successful",
		Message: "We have successfully signed you in to our service. Thank you for choosing us once again.",
	})

	app.writeJSON(w, http.StatusAccepted, serviceJson)
}

func (app *Config) LogItem(w http.ResponseWriter, entry LogPayload) {
	jsonData, _ := json.MarshalIndent(entry, "", "\t")

	request, err := http.NewRequest("POST", "http://logger-service/log", bytes.NewBuffer(jsonData))
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	response, err := client.Do(request)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusAccepted {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusAccepted, jsonResponse{
		Status: true,
		Message: "Logged",
	})
}

func (app *Config) sendMail(msg MailPayload) error {
	jsonData, _ := json.MarshalIndent(msg, "", "\t")

	mailServiceUrl := "http://mail-service/send"

	request, err := http.NewRequest("POST", mailServiceUrl, bytes.NewBuffer(jsonData))

	if err != nil {
		log.Println(err)
		return err
	}

	log.Println("Sending mail")

	client := &http.Client{}
	_, err = client.Do(request)
	
	log.Println(err)

	return err
}

func (app *Config) logEventViaRabbit(w http.ResponseWriter, l LogPayload) {
	err := app.pushToQueue(l.Name, l.Data)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusAccepted, jsonResponse{
		Status: true,
		Message: "logged via RabbitMQ",
	})
}

func (app *Config) pushToQueue(name, msg string) error {
	emitter, err := event.NewEventEmitter(app.Rabbit)
	if err != nil {
		return err
	}

	payload := LogPayload {
		Name: name,
		Data: msg,
	}

	j, _ := json.Marshal(&payload)
	err = emitter.Push(string(j), "log.INFOR")
	
	return err
}

func (app *Config) logItemViaRPC(w http.ResponseWriter, l LogPayload) {
	// get rpc client
	client, err := rpc.Dial("tcp", "logger-service:5001")
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	var result string
	err = client.Call("RPCServer.LogInfo", l, &result)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusAccepted, jsonResponse{
		Status: true,
		Message: result,
	})
}

func (app *Config) LogViaGRPC(w http.ResponseWriter, l LogPayload) {
	// Connect to grpc
	conn, err := grpc.NewClient("logger-service:50001", 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		app.errorJSON(w, err)
		return
	}

	defer conn.Close()

	c := logs.NewLogServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Handle logging with grpc after successful connection
	_, err = c.WriteLog(ctx, &logs.LogRequest{
		LogEntry: &logs.Log{
			Name: l.Name + " GRPC",
			Data: l.Data,
		},
	})

	if err != nil {
		app.errorJSON(w, err)
		return
	}

	app.writeJSON(w, http.StatusAccepted, jsonResponse{
		Status: true,
		Message: "Logged!",
	})
}
