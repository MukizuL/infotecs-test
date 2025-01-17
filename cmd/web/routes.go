package main

import "net/http"

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/send", app.send)
	mux.HandleFunc("GET /api/transactions", app.getLast)
	mux.HandleFunc("GET /api/wallet/{address}/balance", app.getBalance)

	return mux
}
