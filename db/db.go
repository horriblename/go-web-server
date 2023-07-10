package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Chirp struct {
	Id       int    `json:"id"`
	AuthorID int    `json:"author_id"`
	Body     string `json:"body"`
}

type User struct {
	Id             int    `json:"id"`
	Email          string `json:"email"`
	HashedPassword []byte `json:"hashed_password"`
}

type UserDTO struct {
	Id    int    `json:"id"`
	Email string `json:"email"`
}

type DB struct {
	path string
	lock *sync.RWMutex
}

type DBStruct struct {
	Chirps               map[int]Chirp        `json:"chirps"`
	Users                map[int]User         `json:"users"`
	RevokedRefreshTokens map[string]time.Time `json:"revoked_tokens"`
}

var (
	ErrIsDir             = errors.New("the provided database path is a directory")
	ErrUnregisteredEmail = errors.New("email is not registered")
	ErrEmailTaken        = errors.New("email already registered")
	ErrWrongPassword     = bcrypt.ErrMismatchedHashAndPassword
	ErrInvalidUserID     = errors.New("user ID not found in database")
	ErrTokenRevoked      = errors.New("token is revoked")
	ErrChirpNotFound     = errors.New("requested chirp not found")
)

// NewDB creates a new database connection
// and creates the database file if it doesn't exist
func New(path string) (*DB, error) {
	db := DB{path: path, lock: &sync.RWMutex{}}
	return &db, db.ensureDB()
}

func NewDBStruct(chirps []Chirp, users []User) DBStruct {
	dbstruct := DBStruct{make(map[int]Chirp), make(map[int]User), make(map[string]time.Time)}
	for _, chirp := range chirps {
		dbstruct.Chirps[chirp.Id] = chirp
	}
	for _, user := range users {
		dbstruct.Users[user.Id] = user
	}

	return dbstruct
}

func NewUserDTO(data User) UserDTO {
	return UserDTO{data.Id, data.Email}
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

		dbStruct := DBStruct{make(map[int]Chirp), make(map[int]User), make(map[string]time.Time)}
		dat, err := json.Marshal(dbStruct)
		if err != nil {
			return err
		}

		f.Write(dat)
	} else {
		if info.IsDir() {
			return ErrIsDir
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

// Finds and returns a chirp by id. Returns an ErrChirpNotFound if the id does not exist,
// any other error is a databse error.
func (db *DB) GetChirp(id int) (*Chirp, error) {
	dbStruct, err := db.loadDB()

	if err != nil {
		return nil, err
	}

	chirp, ok := dbStruct.Chirps[id]
	if !ok {
		return nil, ErrChirpNotFound
	}

	return &chirp, nil
}

func (db *DB) GetUsers() ([]UserDTO, error) {
	dbStruct, err := db.loadDB()

	if err != nil {
		return nil, err
	}

	users := []UserDTO{}
	for _, user := range dbStruct.Users {
		users = append(users, NewUserDTO(user))
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Id < users[j].Id })

	return users, nil
}

func (db *DB) CreateChirp(userID int, body string) (*Chirp, error) {
	newChirp := Chirp{AuthorID: userID, Body: body}

	dbstruct, err := db.loadDB()
	if err != nil {
		return nil, err
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

	return &newChirp, err
}

func (db *DB) CreateUser(email, password string) (UserDTO, error) {
	newUser := User{Email: email}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return NewUserDTO(newUser), err
	}
	newUser.HashedPassword = hashed

	dbstruct, err := db.loadDB()
	if err != nil {
		return NewUserDTO(newUser), err
	}

	for _, user := range dbstruct.Users {
		if user.Email == email {
			return NewUserDTO(newUser), ErrEmailTaken
		}
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

	return NewUserDTO(newUser), err
}

// Deletes a chirp entry by id. Returns ErrChirpNotFound if it doesn't exist.
func (db *DB) DeleteChirp(id int) error {
	dbStruct, err := db.loadDB()
	if err != nil {
		return err
	}

	if _, ok := dbStruct.Chirps[id]; !ok {
		return ErrChirpNotFound
	}

	delete(dbStruct.Chirps, id)
	db.writeDB(dbStruct)

	return nil
}

func (db *DB) UpdateUser(id int, new_email, new_password string) (*UserDTO, error) {
	dbstruct, err := db.loadDB()
	if err != nil {
		return nil, err
	}

	if _, ok := dbstruct.Users[id]; !ok {
		return nil, fmt.Errorf("%w, missing id: %d", err, id)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(new_password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	updatedUser := User{
		Id:             id,
		Email:          new_email,
		HashedPassword: hashed,
	}
	dbstruct.Users[id] = updatedUser
	db.writeDB(dbstruct)

	// TODO: check for duplicate emails

	dto := NewUserDTO(updatedUser)
	return &dto, nil
}

// validates user and returns the user's details
// If the password is wrong, ErrWrongPassword is returned
// user details is only returned when validation passes
func (db *DB) ValidateUser(email, password string) (*UserDTO, error) {
	dbstruct, err := db.loadDB()
	if err != nil {
		return nil, err
	}

	for _, user := range dbstruct.Users {
		if user.Email == email {
			err = bcrypt.CompareHashAndPassword(user.HashedPassword, []byte(password))
			if err != nil {
				return nil, err
			}

			userDTO := NewUserDTO(user)
			return &userDTO, nil
		}
	}

	return nil, ErrUnregisteredEmail
}

// Checks if a token is marked as revoked. If an error is returned, the token should not be used.
// in particular, if a token is marked as revoked in the database, an ErrTokenRevoked is returned.
func (db *DB) CheckTokenRevocation(token string) error {
	dbStruct, err := db.loadDB()
	if err != nil {
		return err
	}

	if _, ok := dbStruct.RevokedRefreshTokens[token]; ok {
		return ErrTokenRevoked
	}

	return nil
}

func (db *DB) AddTokenRevocation(token string) error {
	dbStruct, err := db.loadDB()
	if err != nil {
		return err
	}

	dbStruct.RevokedRefreshTokens[token] = time.Now()
	err = db.writeDB(dbStruct)
	if err != nil {
		return err
	}

	return nil
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
