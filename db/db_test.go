package db

import (
	"errors"
	"fmt"
	"os"
	"testing"
)

const gDBPath = "/tmp/testing_db.json"

func TestDB(t *testing.T) {
	// remove existing database file
	if _, err := os.Stat(gDBPath); err == nil {
		err := os.Remove(gDBPath)
		if err != nil {
			t.Errorf("could not remove exising DB file at %s: %s", gDBPath, err)
			return
		}
	}

	db, err := New(gDBPath)
	if err != nil {
		t.Errorf("Creating DB: %s", err)
		return
	}
	// defer func() {
	// 	err := os.Remove(gDBPath)
	// 	if err != nil {
	// 		t.Errorf("Error cleaning up DB file %s: %s", gDBPath, err)
	// 	}
	// }()

	{
		chirps, err := db.GetChirps()
		if err != nil {
			t.Errorf("Getting chirps: %s", err)
		}
		if len(chirps) != 0 {
			t.Errorf("Expected to get empty database, got len: %d", len(chirps))
			return
		}
	}

	testAddChirp(db, "first chirp!", 1)
	testAddChirp(db, "second chirp", 2)

	{
	}
}

func testAddChirp(db *DB, content string, expectID int) error {
	expect := Chirp{Id: expectID, Body: content}
	createdChirp, err := db.CreateChirp(content)
	if err != nil {
		return err
	}
	if createdChirp != expect {
		return errors.New(fmt.Sprintf(`Expected chirp to be %+v\n got %+v`, expect, createdChirp))
	}
	chirps, err := db.GetChirps()
	if err != nil {
		return err
	}
	if len(chirps) != expectID {
		return errors.New(fmt.Sprintf(`Expected %d chirps, got %d`, expectID, len(chirps)))
	}
	got := chirps[expectID-1]
	if got != expect {
		return errors.New(fmt.Sprintf(`Expected chirp to be %+v\n got %+v`, expect, got))
	}

	return nil
}

func testAddUser(db *DB, email string, expectID int) error {
	_, err := db.CreateUser(email)
	if err != nil {
		return errors.New(fmt.Sprintf("CreateUser: %s", err))
	}
	users, err := db.GetUsers()
	if err != nil {
		return errors.New(fmt.Sprintf("GetUsers: %s", err))
	}
	if len(users) != expectID {
		return errors.New(fmt.Sprintf(`Expected 1 users, got %d`, len(users)))
	}
	expect := User{Id: expectID, Email: email}
	got := users[expectID-1]
	if got != expect {
		return errors.New(fmt.Sprintf(`Expected user to be %+v\n got %+v`, expect, got))
	}

	return nil
}
