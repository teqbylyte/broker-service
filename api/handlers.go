package main

import (
	"net/http"
)

func (app *Config) Broker(w http.ResponseWriter, r *http.Request) {
	payload := jsonResponse{
		Status:  true,
		Message: "Hit the broker",
	}

	app.writeJSON(w, http.StatusOK, payload)
}
