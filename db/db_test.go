package db

import (
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

	{
		content := "first chirp!"
		_, err := db.CreateChirp(content)
		if err != nil {
			t.Errorf("CreateChirp: %s", err)
			return
		}
		chirps, err := db.GetChirps()
		if err != nil {
			t.Errorf("GetChirps: %s", err)
			return
		}
		if len(chirps) != 1 {
			t.Errorf(`Expected 1 chirps, got %+v`, chirps)
			return
		}
		expect := Chirp{Id: 1, Body: content}
		if chirps[0] != expect {
			t.Errorf(`Expected chirp to be %+v\n got %+v`, expect, chirps[0])
			return
		}
	}

	{
		content := "second"
		_, err := db.CreateChirp(content)
		if err != nil {
			t.Errorf("CreateChirp: %s", err)
			return
		}
		chirps, err := db.GetChirps()
		if err != nil {
			t.Errorf("GetChirps: %s", err)
			return
		}
		if len(chirps) != 2 {
			t.Errorf(`Expected 2 chirps, got %d`, len(chirps))
			return
		}
		expect := Chirp{Id: 2, Body: content}
		if chirps[1] != expect {
			t.Errorf(`Expected chirp to be %+v\n got %+v`, expect, chirps[0])
			return
		}
	}
}
