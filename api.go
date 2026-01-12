package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type api struct {
	addr   string
	users  []User
	nextID int
}

func (a *api) insertUser(u User) error {
	if u.FirstName == "" {
		return errors.New("first name is required")
	}
	if u.LastName == "" {
		return errors.New("last name is required")
	}

	for _, existingUser := range a.users {
		if existingUser.FirstName == u.FirstName && existingUser.LastName == u.LastName {
			return errors.New("user with this first name and last name already exists")
		}
	}

	a.users = append(a.users, u)
	return nil
}

func (a *api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello from ServeHTTP\n"))
}

func (a *api) getUsersHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Has("firstName") || q.Has("lastName") {
		a.getUsersByHandlerQuery(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(a.users)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *api) getUserByIdHandler(w http.ResponseWriter, r *http.Request) {
	userId := r.PathValue("id")

	// Search for the user with matching ID
	for _, user := range a.users {
		if user.ID == userId {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			err := json.NewEncoder(w).Encode(user)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			return
		}
	}

	// User not found
	http.Error(w, "user not found", http.StatusNotFound)
}

func (a *api) deleteUserByIdHandler(w http.ResponseWriter, r *http.Request) {
	userId := r.PathValue("id")

	for i, user := range a.users {
		if user.ID == userId {
			a.users = append(a.users[:i], a.users[i+1:]...)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	http.Error(w, "user not found", http.StatusNotFound)
}

func (a *api) createUserHandler(w http.ResponseWriter, r *http.Request) {
	var payload User
	err := json.NewDecoder(r.Body).Decode(&payload)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	u := User{
		ID:        strconv.Itoa(a.nextID),
		FirstName: payload.FirstName,
		LastName:  payload.LastName,
	}
	a.nextID++

	err = a.insertUser(u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (a *api) updateUserByIdHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")

	// 1) Parse patch body
	var patch struct {
		FirstName *string `json:"firstName"`
		LastName  *string `json:"lastName"`
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // reject fields other than firstName/lastName

	if err := dec.Decode(&patch); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	// Optional: ensure body isn't empty {} (no-op patch)
	if patch.FirstName == nil && patch.LastName == nil {
		http.Error(w, "no fields to update", http.StatusBadRequest)
		return
	}

	// 2) Find user + apply patch
	for i := range a.users {
		if a.users[i].ID == userID {

			// Validate + apply firstName if provided
			if patch.FirstName != nil {
				if *patch.FirstName == "" {
					http.Error(w, "firstName cannot be empty", http.StatusBadRequest)
					return
				}
				a.users[i].FirstName = *patch.FirstName
			}

			// Validate + apply lastName if provided
			if patch.LastName != nil {
				if *patch.LastName == "" {
					http.Error(w, "lastName cannot be empty", http.StatusBadRequest)
					return
				}
				a.users[i].LastName = *patch.LastName
			}

			// 3) Return updated user (common choice)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(a.users[i])
			return
		}
	}

	// 4) Not found
	http.Error(w, "user not found", http.StatusNotFound)
}

func (a *api) getUsersByHandlerQuery(w http.ResponseWriter, r *http.Request) {
	firstName := r.URL.Query().Get("firstName")
	lastName := r.URL.Query().Get("lastName")
	var users []User
	for _, user := range a.users {
		if firstName != "" && user.FirstName != firstName {
			continue
		}
		if lastName != "" && user.LastName != lastName {
			continue
		}
		users = append(users, user)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(users)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
