package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

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

// getUserByIdHandler gets a user by id from the database
func (a *api) getUserByIdHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	userId := r.PathValue("id")

	u, src, err := a.getUserByIdDedupe(ctx, userId)
	if err != nil {
		// 1) timeout / canceled
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			http.Error(w, "request timeout/canceled", http.StatusGatewayTimeout)
			return
		}

		// 2) not found
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		// 3) everything else
		http.Error(w, "failed to get user", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Source", src)
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(u)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// getUserByIdDedupe helps to prevent duplicate requests for the same resource
func (a *api) getUserByIdDedupe(ctx context.Context, id string) (User, string, error) {
	// 1) cache first
	if u, err := a.getUserFromCache(id); err == nil {
		return u, "cache", nil
	}

	// 2) inflight gate
	a.inflightMu.Lock()
	if ch, ok := a.inflight[id]; ok {
		// follower: someone else is fetching
		a.inflightMu.Unlock()

		select {
		case res := <-ch:
			// leader already did DB work
			if res.err == nil {
				return res.user, "shared", nil
			}
			return User{}, "shared", res.err
		case <-ctx.Done():
			return User{}, "shared", ctx.Err()
		}
	}

	// leader: create waiting room
	ch := make(chan fetchResult, 1)
	a.inflight[id] = ch
	a.inflightMu.Unlock()

	// Ensure all followers are released no matter what
	defer func() {
		a.inflightMu.Lock()
		delete(a.inflight, id)
		a.inflightMu.Unlock()
		close(ch)
	}()

	// 3) do DB work
	u, err := a.getUserById(ctx, id)
	if err == nil {
		// fill cache (use your TTL)
		a.setUserCache(id, u, 30*time.Second)
	}

	// 4) broadcast to followers
	ch <- fetchResult{user: u, err: err}

	if err != nil {
		return User{}, "db", err
	}
	return u, "db", nil
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

	// Invalidate cache for this user
	a.invalidateUserCache(userId)

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

	// Invalidate cache for this user (will be repopulated on next GET)
	a.invalidateUserCache(u.ID)

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
