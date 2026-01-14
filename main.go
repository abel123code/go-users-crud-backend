package main

import (
	"log"
	"net/http"
)

func route(api *api) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", api.healthHandler)
	mux.HandleFunc("GET /users", api.getUsersHandler)
	mux.HandleFunc("POST /users", api.createUserHandler)
	mux.HandleFunc("GET /users/{id}", api.getUserByIdHandler)
	mux.HandleFunc("DELETE /users/{id}", api.deleteUserByIdHandler)
	mux.HandleFunc("PATCH /users/{id}", api.updateUserByIdHandler)

	var h http.Handler = mux

	h = loggingMiddleware(h)
	h = requestIDMiddleware(h)
	h = recoverMiddleware(h)

	return h
}

func main() {
	db := openDB()
	defer db.Close()

	if err := initSchema(db); err != nil {
		log.Fatal(err)
	}

	api := &api{addr: ":8080", db: db}

	srv := &http.Server{
		Addr:    api.addr,
		Handler: route(api),
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
