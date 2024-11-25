package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"

	"github.com/gorilla/mux"
)

type LocalStorage struct {
	Config  *os.File
	Storage []User
}

func (s *LocalStorage) CardsUpToDate() {

	for j, _ := range s.Storage {

		user := &s.Storage[j]
		// todaysDate := time.Now().Format("2006.01.02")
		for i, _ := range user.Tracks {
			track := &user.Tracks[i]
			track.FromLanguage.VerifyTestStatuses(track.Settings.DaylyTestTries)
			track.ToLanguage.VerifyTestStatuses(track.Settings.DaylyTestTries)
			track.Writing.VerifyTestStatuses(track.Settings.DaylyTestTries)
			track.Listening.VerifyTestStatuses(track.Settings.DaylyTestTries)

			for x, _ := range track.Storage {

				card := &track.Storage[x]

				card.ToLanguage.VerifyDates()
				card.FromLanguage.VerifyDates()
				card.Writing.VerifyDates()
				card.Listening.VerifyDates()

			}

			track.MissingTests()

		}

	}

	s.UpdateData()
}

func (s *LocalStorage) GetTrack(r *http.Request) (*Track, error) {
	user, err := s.GetUser(r)
	if err != nil {
		return nil, nil
	}

	key, err := GetKey(r)
	if err != nil {
		return nil, nil
	}

	index := slices.IndexFunc(user.Tracks, func(t Track) bool {
		return t.Name == key
	})

	if index != -1 {

		user.TracksKeys = append([]string{key}, slices.DeleteFunc(user.TracksKeys, func(keyToCheck string) bool {
			return keyToCheck == key
		})...)

		return &user.Tracks[index], nil
	}

	return nil, fmt.Errorf("Track does't exist")

}

func (s *LocalStorage) GetCard(r *http.Request) (*Card, error) {
	track, err := s.GetTrack(r)

	if err != nil {
		return nil, err
	}

	id, err := getCardID(r)
	if err != nil {
		return nil, err
	}

	i := slices.IndexFunc(track.Storage, func(card Card) bool {
		return card.ID == id
	})

	if i == -1 {
		return nil, fmt.Errorf("Track does't exist")
	}
	return &track.Storage[i], nil
}

func (s *LocalStorage) DeleteCard(r *http.Request) (*Card, error) {

	trackKey, err := GetKey(r)
	if err != nil {
		return nil, fmt.Errorf("Couldn get track key")
	}

	userID, err := getID(r)
	if err != nil {
		return nil, fmt.Errorf("Couldn get user ID")
	}

	cardID, err := getCardID(r)
	if err != nil {
		return nil, fmt.Errorf("Card wasn't found")
	}

	user := &s.Storage[slices.IndexFunc(s.Storage, func(user User) bool {
		return user.ID == userID
	})]

	track := &user.Tracks[slices.IndexFunc(user.Tracks, func(track Track) bool {
		return track.Name == trackKey
	})]

	track.Storage = slices.DeleteFunc(track.Storage, func(card Card) bool {
		return card.ID == cardID
	})

	track.MissingTests()

	s.UpdateData()
	return nil, nil
}

func (s *LocalStorage) GetUser(r *http.Request) (*User, error) {
	id, err := getID(r)

	if err != nil {
		return nil, err
	}

	i := slices.IndexFunc(s.Storage, func(user User) bool {
		return user.ID == id
	})
	if i == -1 {
		return nil, fmt.Errorf("User doesn't exist")
	}

	return &s.Storage[i], nil
}

func (s *LocalStorage) GetUserByID(id int) (*User, error) {

	i := slices.IndexFunc(s.Storage, func(user User) bool {
		return user.ID == id
	})
	if i == -1 {
		return nil, fmt.Errorf("User doesn't exist")
	}

	return &s.Storage[i], nil
}

func (s *LocalStorage) GetUserByToken(token string) (*User, error) {

	i := slices.IndexFunc(s.Storage, func(user User) bool {
		return user.UserName == token
	})
	if i == -1 {
		return nil, fmt.Errorf("User doesn't exist")
	}

	return &s.Storage[i], nil
}

func getID(r *http.Request) (int, error) {

	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)

	if err != nil {
		return id, fmt.Errorf("Invalid id given %s", idStr)
	}
	return id, nil

}

func testname(r *http.Request) (string, error) {

	name := mux.Vars(r)["testName"]

	return name, nil

}

func GetKey(r *http.Request) (string, error) {
	idStr := mux.Vars(r)["key"]

	return idStr, nil

}

func getCardID(r *http.Request) (int, error) {
	idStr := mux.Vars(r)["cardID"]
	id, err := strconv.Atoi(idStr)

	if err != nil {
		return id, fmt.Errorf("Invalid id given %s", idStr)
	}
	return id, nil

}

func (s *LocalStorage) CreateAccount(newUser User) (*User, error) {
	if len(s.Storage) == 0 {
		newUser.ID = 0
	} else {

		newUser.ID = s.Storage[len(s.Storage)-1].ID + 1
	}

	index := slices.IndexFunc(s.Storage, func(user User) bool {

		return user.UserName == newUser.UserName

	})

	if index != -1 {
		return nil, fmt.Errorf("Username taken")
	}

	s.Storage = append(s.Storage, newUser)
	go s.WriteToStorage()
	return &s.Storage[slices.IndexFunc(s.Storage, func(user User) bool {
		return user.ID == newUser.ID
	})], nil

}

func (s *LocalStorage) CreateTrack(track Track, r *http.Request) error {

	user, err := s.GetUser(r)

	if err != nil {
		return err
	}

	user.Tracks = append([]Track{track}, user.Tracks...)
	user.TracksKeys = append([]string{track.Name}, user.TracksKeys...)

	if err := s.WriteToStorage(); err != nil {
		fmt.Println(err)
	}
	return nil
}

func (s *LocalStorage) UpdateData() {
	s.WriteToStorage()
	storage, _ := OpenStorage("./storage.json")
	s = &storage

}

func OpenStorage(path string) (LocalStorage, error) {
	storage, err := os.OpenFile("./storage.json", os.O_RDWR, 0644)

	if err != nil {
		return LocalStorage{}, err
	}

	oldStorageData, err := io.ReadAll(storage)

	if err != nil {
		return LocalStorage{}, err
	}
	var storageData []User
	err = json.Unmarshal(oldStorageData, &storageData)

	if err != nil {
		return LocalStorage{}, err
	}

	return LocalStorage{
		Storage: storageData,
		Config:  storage,
	}, nil
}

func (s *LocalStorage) WriteToStorage() error {

	users, err := json.Marshal(s.Storage)

	if err != nil {
		return err
	}
	err = s.Config.Truncate(0)

	if err != nil {
		return err
	}
	_, err = s.Config.WriteAt(users, 0)
	if err != nil {
		return err
	}
	err = s.Config.Sync()

	if err != nil {
		return err
	}
	return nil
}

type Authentification struct {
	Id       int    `json:"id"`
	UserName string `jsom:"userName"`
	EMail    string `jsom:"eMail"`
	Password string `jsom:"password"`
	Token    string `jsom:"token"`
}

var testUserNames = []string{"userTest", "studyTest", "quizeTest", "routerTest"}

func (s *LocalStorage) SingUp(req SingUp) (Authentification, error) {

	if id := (slices.IndexFunc(s.Storage, func(user User) bool {
		return user.UserName == req.Username
	})); id > -1 {

		if slices.Contains(testUserNames, req.Username) {

			s.Storage = slices.DeleteFunc(s.Storage, func(user User) bool {
				return user.UserName == req.Username
			})
			s.UpdateData()

		}
	}

	user, err := s.CreateAccount(*NewUser(req.FirstName, req.LastName, req.EMail, req.Username, req.Password))

	if err != nil {
		return Authentification{}, err
	}
	return Authentification{user.ID, req.Username, req.EMail, req.Password, strconv.Itoa(user.ID)}, nil
}

func (s *LocalStorage) LogIn(req LogInReq) (Authentification, error) {
	index := slices.IndexFunc(s.Storage, func(user User) bool {
		return user.EMail == req.UsernameEMail || user.UserName == req.UsernameEMail
	})

	if index == -1 {
		return Authentification{}, fmt.Errorf("User does't exist")
	}

	if s.Storage[index].Password == req.Password {
		user := s.Storage[index]
		return Authentification{user.ID, user.UserName, user.EMail, user.Password, user.Token}, nil
	}
	return Authentification{}, fmt.Errorf("Incorect password")
}
