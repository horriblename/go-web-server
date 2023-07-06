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

type User struct {
	Id    int    `json:"id"`
	Email string `json:"email"`
}

type DB struct {
	path string
	lock *sync.RWMutex
}

type DBStruct struct {
	Chirps map[int]Chirp `json:"chirps"`
	Users  map[int]User  `json:"users"`
}

var errIsDir error = errors.New("the provided database path is a directory")

// NewDB creates a new database connection
// and creates the database file if it doesn't exist
func New(path string) (*DB, error) {
	db := DB{path: path, lock: &sync.RWMutex{}}
	return &db, db.ensureDB()
}

func NewDBStruct(chirps []Chirp, users []User) DBStruct {
	dbstruct := DBStruct{make(map[int]Chirp), make(map[int]User)}
	for _, chirp := range chirps {
		dbstruct.Chirps[chirp.Id] = chirp
	}
	for _, user := range users {
		dbstruct.Users[user.Id] = user
	}

	return dbstruct
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

		dbStruct := DBStruct{make(map[int]Chirp), make(map[int]User)}
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
	dbStruct, err := db.loadDB()

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

func (db *DB) GetUsers() ([]User, error) {
	dbStruct, err := db.loadDB()

	if err != nil {
		return nil, err
	}

	users := []User{}
	for _, user := range dbStruct.Users {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Id < users[j].Id })

	return users, nil
}

// loadDB reads the database file into memory
func (db *DB) loadDB() (DBStruct, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	var dbStruct DBStruct

	f, err := os.Open(db.path)
	defer f.Close()

	if err != nil {
		return dbStruct, err
	}

	decoder := json.NewDecoder(f)
	err = decoder.Decode(&dbStruct)

	return dbStruct, err
}

func (db *DB) CreateChirp(body string) (Chirp, error) {
	newChirp := Chirp{Body: body}

	dbstruct, err := db.loadDB()
	if err != nil {
		return newChirp, err
	}

	maxID := 0
	for id := range dbstruct.Chirps {
		if id > maxID {
			maxID = id
		}
	}
	newChirp.Id = maxID + 1
	dbstruct.Chirps[newChirp.Id] = newChirp

	err = db.writeDB(dbstruct)

	return newChirp, err
}

func (db *DB) CreateUser(email string) (User, error) {
	newUser := User{Email: email}

	dbstruct, err := db.loadDB()
	if err != nil {
		return newUser, err
	}

	maxID := 0
	for id := range dbstruct.Users {
		if id > maxID {
			maxID = id
		}
	}
	newUser.Id = maxID + 1
	dbstruct.Users[newUser.Id] = newUser

	err = db.writeDB(dbstruct)

	return newUser, err

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
