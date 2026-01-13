package main

import (
	"database/sql"
	"encoding/json"
	"strconv"

	//"errors"
	"net/http"
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
	users, err := a.listUsers()
	if err != nil {
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
	userId := r.PathValue("id")
	user, err := a.getUserById(userId)
	if err != nil {
		http.Error(w, "failed to get user", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(user)
	if err != nil {
		http.Error(w, "failed to encode user", http.StatusInternalServerError)
	}
}

// deleteUserByIdHandler deletes a user by id from the database
func (a *api) deleteUserByIdHandler(w http.ResponseWriter, r *http.Request) {
	userId := r.PathValue("id")

	deleted, err := a.deleteUserById(userId)
	if err != nil {
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

	u, err := a.createUser(payload.FirstName, payload.LastName)
	if err != nil {
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(u)
}

// updateUserByIdHandler updates a user by id from the database
func (a *api) updateUserByIdHandler(w http.ResponseWriter, r *http.Request) {
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

	u, updated, err := a.updateUserByID(id, patch.FirstName, patch.LastName)
	if err != nil {
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
