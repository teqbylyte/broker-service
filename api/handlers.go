package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

type RequestPayload struct {
	Action string `json:"action"`
	Auth AuthPayload `json:"auth,omitempty"`
}

type AuthPayload struct {
	Email string `json:"email"`
	Password string `json:"password"`
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

	log.Println("Handling")

	err := app.readJSON(w, r, &requestPayload)
	if err != nil {
		log.Println(err)
		app.errorJSON(w, err)
		return
	}

	log.Println("Get request body")

	switch requestPayload.Action {
	case "auth":
		app.Authenticate(w, requestPayload.Auth)
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

	log.Println("Called auth service")

	defer res.Body.Close()

	// check status code returned
	if res.StatusCode == http.StatusUnauthorized {
		app.errorJSON(w, errors.New("invalid credentials"))
		return
	} else if res.StatusCode != http.StatusAccepted {
		app.errorJSON(w, errors.New("error calling auth service"))
		return
	}

	log.Println("Checked status from auth service")

	// create a variable
	var serviceJson jsonResponse
	err = json.NewDecoder(res.Body).Decode(&serviceJson)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	log.Println("Get auth error")

	if !serviceJson.Status {
		app.errorJSON(w, errors.New(serviceJson.Message), http.StatusUnauthorized)
		return
	} 
	
	log.Println("Successful auth")

	app.writeJSON(w, http.StatusAccepted, serviceJson)
}
