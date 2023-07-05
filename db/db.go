package db

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"sync"
)

type Chirp struct {
	Id   int    `json:"id"`
	Body string `json:"body"`
}

type DB struct {
	path string
	lock *sync.RWMutex
}

type DBStruct struct {
	Chirps map[int]Chirp `json:"chirps"`
}

var errIsDir error = errors.New("the provided database path is a directory")

// NewDB creates a new database connection
// and creates the database file if it doesn't exist
func New(path string) (*DB, error) {
	db := DB{path: path, lock: &sync.RWMutex{}}
	return &db, db.ensureDB()
}

// creates database file if it doesn't exist
func (db *DB) ensureDB() error {
	info, err := os.Stat(db.path)

	// file doesn't exist
	if err != nil {
		f, err := os.Create(db.path)
		if err != nil {
			return err
		}
		defer f.Close()

		dbStruct := DBStruct{make(map[int]Chirp)}
		dat, err := json.Marshal(dbStruct)
		if err != nil {
			return err
		}

		f.Write(dat)
	} else {
		if info.IsDir() {
			return errIsDir
		}
	}

	return nil
}

// GetChirps returns all chirps in the database
func (db *DB) GetChirps() ([]Chirp, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	f, err := os.Open(db.path)
	defer f.Close()
	if err != nil {
		return nil, err
	}

	var dbStruct DBStruct

	decoder := json.NewDecoder(f)
	err = decoder.Decode(&dbStruct)
	if err != nil {
		return nil, err
	}

	chirps := []Chirp{}
	for _, chirp := range dbStruct.Chirps {
		chirps = append(chirps, chirp)
	}
	sort.Slice(chirps, func(i, j int) bool { return chirps[i].Id < chirps[j].Id })

	return chirps, nil
}

func (db *DB) CreateChirp(body string) (Chirp, error) {
	newChirp := Chirp{Body: body}

	chirps, err := db.GetChirps()
	if err != nil {
		return newChirp, err
	}

	if len(chirps) > 0 {
		newChirp.Id = chirps[len(chirps)-1].Id + 1
	} else {
		newChirp.Id = 1
	}
	newChirp.Id = len(chirps) + 1
	chirps = append(chirps, newChirp)

	dbStruct := DBStruct{
		Chirps: make(map[int]Chirp),
	}
	for _, chirp := range chirps {
		dbStruct.Chirps[chirp.Id] = chirp
	}
	dbStruct.Chirps[newChirp.Id] = newChirp

	err = db.writeDB(dbStruct)

	return newChirp, err
}

// writes the database file to disk
func (db *DB) writeDB(dbStruct DBStruct) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	f, err := os.OpenFile(db.path, os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	return encoder.Encode(dbStruct)
}
