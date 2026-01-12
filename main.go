package main

import (
	"log"
	"net/http"
)

func route(api *api) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users", api.getUsersHandler)
	mux.HandleFunc("POST /users", api.createUserHandler)
	mux.HandleFunc("GET /users/{id}", api.getUserByIdHandler)
	mux.HandleFunc("DELETE /users/{id}", api.deleteUserByIdHandler)
	mux.HandleFunc("PATCH /users/{id}", api.updateUserByIdHandler)
	return mux
}

func main() {
	api := &api{addr: ":8080", nextID: 1, users: []User{}}

	srv := &http.Server{
		Addr:    api.addr,
		Handler: route(api),
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
