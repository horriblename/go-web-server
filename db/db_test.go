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

	assertOk := func(err error) {
		if err != nil {
			t.Errorf("%s", err)
		}
	}

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

	assertOk(testAddChirp(db, "first chirp!", 1))
	assertOk(testAddChirp(db, "second chirp", 2))

	assertOk(testAddUser(db, "x@ymail.com", "U@*#PFOcj mp", 1))
	assertOk(testAddUser(db, "abc@dmail.com", "10f9j", 2))
	err = testAddUser(db, "x@ymail.com", ";alksdjf", -1)
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("expected %s, got %s", ErrEmailTaken, err)
	}

	assertOk(testValidatePassword(db, "x@ymail.com", "U@*#PFOcj mp", 1, true))
	assertOk(testValidatePassword(db, "abc@dmail.com", "10f9j", 2, true))
	err = testValidatePassword(db, "x@ymail.com", "wrong password", -1, false)
	if !errors.Is(err, ErrWrongPassword) {
		t.Errorf("expected ErrWrongPassword, got %s", err)
	}

	assertOk(testUpdateUser(db, 1, "new@ymail.com", "U@*#PFOcj mp"))
	assertOk(testUpdateUser(db, 2, "new@dmail.com", "new_password"))

	revokedToken := "revoked_token"
	assertOk(db.AddTokenRevocation(revokedToken))
	err = db.CheckTokenRevocation(revokedToken)
	if err != ErrTokenRevoked {
		t.Errorf(`Expected error to be %s, got %s`, ErrTokenRevoked, err)
	}

	assertOk(db.CheckTokenRevocation("not_revoked"))
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

func testAddUser(db *DB, email, password string, expectID int) error {
	_, err := db.CreateUser(email, password)
	if err != nil {
		return fmt.Errorf("CreateUser: %w", err)
	}
	users, err := db.GetUsers()
	if err != nil {
		return fmt.Errorf("GetUsers: %w", err)
	}
	if len(users) != expectID {
		return fmt.Errorf(`Expected 1 users, got %d`, len(users))
	}
	expect := UserDTO{Id: expectID, Email: email}
	got := users[expectID-1]
	if got != expect {
		return fmt.Errorf(`Expected user to be %+v\n got %+v`, expect, got)
	}

	return nil
}

func testValidatePassword(db *DB, email, password string, expectID int, expectPass bool) error {
	// test that passwords work
	user, err := db.ValidateUser(email, password)
	if expectPass {
		if err != nil {
			return fmt.Errorf("expected no error, got %w", err)
		}
	} else {
		if !errors.Is(err, ErrWrongPassword) {
			return fmt.Errorf("expected ErrWrongPassword, got %w", err)
		}
		return err
	}

	if user.Email != email {
		return fmt.Errorf("expected email to be %s, got %s", email, user.Email)
	}

	if user.Id != expectID {
		return fmt.Errorf("expected id to be %d, got %d", expectID, user.Id)
	}

	return nil
}

func testUpdateUser(db *DB, id int, new_email, new_password string) error {
	_, err := db.UpdateUser(id, new_email, new_password)
	if err != nil {
		return err
	}

	dbstruct, err := db.loadDB()
	if err != nil {
		return err
	}

	got := dbstruct.Users[id].Email
	if got != new_email {
		return fmt.Errorf("expected updated email to be %s, got %s", new_email, got)
	}

	_, err = db.ValidateUser(new_email, new_password)
	if err != nil {
		return fmt.Errorf("validating updated password: %w", err)
	}

	return nil
}
