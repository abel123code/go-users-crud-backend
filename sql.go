package main

import (
	"context"
	"database/sql"
)

// createUser creates a new user in the database
func (a *api) createUser(ctx context.Context, firstName, lastName string) (User, error) {
	var u User
	err := a.db.QueryRowContext(ctx,
		`INSERT INTO users (first_name, last_name)
		 VALUES ($1, $2)
		 RETURNING id::text, first_name, last_name, created_at`,
		firstName, lastName,
	).Scan(&u.ID, &u.FirstName, &u.LastName, &u.CreatedAt)

	return u, err
}

// listUsers lists all users in the database
func (a *api) listUsers(ctx context.Context) ([]User, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id::text, first_name, last_name, created_at
		FROM users
		ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User

	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// getUserById gets a user by id from the database
func (a *api) getUserById(ctx context.Context, id string) (User, error) {
	var u User
	err := a.db.QueryRowContext(ctx,
		`SELECT id::text, first_name, last_name, created_at
		FROM users
		WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.FirstName, &u.LastName, &u.CreatedAt)
	return u, err
}

// deleteUserById deletes a user by id from the database
func (a *api) deleteUserById(ctx context.Context, id string) (bool, error) {
	res, err := a.db.ExecContext(ctx,
		`DELETE FROM users WHERE id = $1`,
		id,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// updateUserByID updates a user by id from the database
func (a *api) updateUserByID(
	ctx context.Context,
	id int64,
	firstName *string,
	lastName *string,
) (User, bool, error) {

	query := `
		UPDATE users
		SET
			first_name = COALESCE($2, first_name),
			last_name  = COALESCE($3, last_name)
		WHERE id = $1
		RETURNING id::text, first_name, last_name, created_at
	`

	var u User
	err := a.db.QueryRowContext(ctx, query, id, firstName, lastName).
		Scan(&u.ID, &u.FirstName, &u.LastName, &u.CreatedAt)

	if err == sql.ErrNoRows {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}

	return u, true, nil
}
