package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

// --- Test Harness ---

func openTestDb(t *testing.T) *sql.DB {
	t.Helper()

	_ = godotenv.Load()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL or DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("db.Ping: %v", err)
	}

	return db
}

func newTestServer(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()

	db := openTestDb(t)

	api := &api{
		addr: ":0",
		db:   db,
	}

	ts := httptest.NewServer(route(api))
	return ts, db
}

func createUser(t *testing.T, baseURL string, first string, last string) User {
	t.Helper()

	payload := fmt.Sprintf(`{"firstName":"%s","lastName":"%s"}`, first, last)
	req, err := http.NewRequest("POST", baseURL+"/users", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /users: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var u User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return u
}

func TestHealth(t *testing.T) {
	ts, db := newTestServer(t)
	defer ts.Close()
	defer db.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestCreateUser(t *testing.T) {
	t.Skip("skipping this test for now")
	ts, db := newTestServer(t)
	defer ts.Close()
	defer db.Close()

	u := createUser(t, ts.URL, "James", "Bond")

	if u.ID == "" {
		t.Fatal("expected created user to have ID")
	}
}

func TestGetUsersByID(t *testing.T) {
	t.Skip("skipping this test for now")
	ts, db := newTestServer(t)
	defer ts.Close()
	defer db.Close()

	u := createUser(t, ts.URL, "Joseph", "Stalin")

	resp, err := http.Get(fmt.Sprintf("%s/users/%s", ts.URL, u.ID))
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetUsersByIDNotFound(t *testing.T) {
	ts, db := newTestServer(t)
	defer ts.Close()
	defer db.Close()

	resp, err := http.Get(fmt.Sprintf("%s/users/9999", ts.URL))
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
