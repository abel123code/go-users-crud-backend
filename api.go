package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
	//"strconv"
)

type api struct {
	addr string
	db   *sql.DB
}

func (a *api) healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := a.db.Ping(); err != nil {
		http.Error(w, "db not reachable", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// Previously used to insert users into a slice in memory (when still using local storage)
// func (a *api) insertUser(u User) error {
// 	if u.FirstName == "" {
// 		return errors.New("first name is required")
// 	}
// 	if u.LastName == "" {
// 		return errors.New("last name is required")
// 	}

// 	for _, existingUser := range a.users {
// 		if existingUser.FirstName == u.FirstName && existingUser.LastName == u.LastName {
// 			return errors.New("user with this first name and last name already exists")
// 		}
// 	}

// 	a.users = append(a.users, u)
// 	return nil
// }

func (a *api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello from ServeHTTP\n"))
}

// getUsersHandler lists all users in the database
func (a *api) getUsersHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	users, err := a.listUsers(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			http.Error(w, "request timeout/canceled", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "failed to list users", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(users)
	if err != nil {
		http.Error(w, "failed to encode users", http.StatusInternalServerError)
	}
}

type userResult struct {
	user User
	src  string
	err  error
}

// getUserFromCache gets a user from the cache
func (api *api) getUserFromCache(ctx context.Context, id string) (User, error) {
	select {
	case <-time.After(20 * time.Millisecond):
		// simulate cache latency and miss most of the time
		return User{}, fmt.Errorf("cache miss")
	case <-ctx.Done():
		return User{}, ctx.Err()
	}
}

// getUserByIdHandler gets a user by id from the database
func (a *api) getUserByIdHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	userId := r.PathValue("id")

	resCh := make(chan userResult, 2)

	go func() {
		user, err := a.getUserFromCache(ctx, userId)
		resCh <- userResult{user: user, src: "cache", err: err}
	}() // go routine 1 to retrieve user from cache

	go func() {
		user, err := a.getUserById(ctx, userId)
		resCh <- userResult{user: user, src: "db", err: err}
	}() // go routine 2 to retrieve user from database

	// Wait for the first *successful* answer (or timeout)
	var firstErr error
	for i := 0; i < 2; i++ {
		select {
		case res := <-resCh:
			if res.err == nil {
				cancel() // stop the other request path
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Source", res.src)
				w.WriteHeader(http.StatusOK)

				if err := json.NewEncoder(w).Encode(res.user); err != nil {
					http.Error(w, "failed to encode response", http.StatusInternalServerError)
				}
				return
			}
			firstErr = res.err
		case <-ctx.Done():
			http.Error(w, "timeout: "+ctx.Err().Error(), http.StatusGatewayTimeout)
			return
		}
	}

	// Both failed
	http.Error(w, "failed: "+firstErr.Error(), http.StatusNotFound)
}

// deleteUserByIdHandler deletes a user by id from the database
func (a *api) deleteUserByIdHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	userId := r.PathValue("id")

	deleted, err := a.deleteUserById(ctx, userId)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			http.Error(w, "request timeout/canceled", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "failed to delete user", http.StatusInternalServerError)
		return
	}
	if !deleted {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// createUserHandler creates a new user in the database
func (a *api) createUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	var payload struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if payload.FirstName == "" || payload.LastName == "" {
		http.Error(w, "firstName and lastName are required", http.StatusBadRequest)
		return
	}

	u, err := a.createUser(ctx, payload.FirstName, payload.LastName)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			http.Error(w, "request timeout/canceled", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(u)
}

// updateUserByIdHandler updates a user by id from the database
func (a *api) updateUserByIdHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	//this pointers allow us to update field that are provided.
	//if the field is not provided, it will be nil and not updated.
	var patch struct {
		FirstName *string `json:"firstName"`
		LastName  *string `json:"lastName"`
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&patch); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if patch.FirstName == nil && patch.LastName == nil {
		http.Error(w, "no fields to update", http.StatusBadRequest)
		return
	}

	if patch.FirstName != nil && *patch.FirstName == "" {
		http.Error(w, "firstName cannot be empty", http.StatusBadRequest)
		return
	}
	if patch.LastName != nil && *patch.LastName == "" {
		http.Error(w, "lastName cannot be empty", http.StatusBadRequest)
		return
	}

	u, updated, err := a.updateUserByID(ctx, id, patch.FirstName, patch.LastName)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			http.Error(w, "request timeout/canceled", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "failed to update user", http.StatusInternalServerError)
		return
	}
	if !updated {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(u)
}

// func (a *api) getUsersByHandlerQuery(w http.ResponseWriter, r *http.Request) {
// 	firstName := r.URL.Query().Get("firstName")
// 	lastName := r.URL.Query().Get("lastName")
// 	var users []User
// 	for _, user := range a.users {
// 		if firstName != "" && user.FirstName != firstName {
// 			continue
// 		}
// 		if lastName != "" && user.LastName != lastName {
// 			continue
// 		}
// 		users = append(users, user)
// 	}
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusOK)
// 	err := json.NewEncoder(w).Encode(users)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// }
