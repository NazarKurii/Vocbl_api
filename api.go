package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

type APIServer struct {
	listenAddr string
	dataBase   LocalStorage
}

type APIError struct {
	Error string `json:"error"`
}

type apiFunc func(http.ResponseWriter, *http.Request) error

func makeHTTPHandleFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			WriteJSON(w, http.StatusBadRequest, APIError{Error: err.Error()})
		}
	}
}

func NewAPISErver(listenAddr string, store LocalStorage) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		dataBase:   store,
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // Allow all origins
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			// Handle preflight request
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) Run() {

	router := mux.NewRouter()
	s.dataBase, _ = OpenStorage("./storage.json")

	router.HandleFunc("/audio", handleAudioRequest)
	router.HandleFunc("/verifyUserName", makeHTTPHandleFunc(s.handleVerifyUserName))
	router.HandleFunc("/acceptCokies", makeHTTPHandleFunc(s.cookiesAccepted))
	router.HandleFunc("/register", makeHTTPHandleFunc(s.handleRegister))
	router.HandleFunc("/getUserByToken", makeHTTPHandleFunc(s.handleGetUserByToken))
	router.HandleFunc("/getUsersToken", makeHTTPHandleFunc(s.handleGetUsersToken))
	router.HandleFunc("/user", makeHTTPHandleFunc(s.handleUser))
	router.HandleFunc("/user/{id}", makeHTTPHandleFunc(s.handeUser))
	router.HandleFunc("/newCardData/{fromLanguage}-{toLanguage}/{expretion}", makeHTTPHandleFunc(s.handleGetNewCardData))
	router.HandleFunc("/user/{id}/track/{key}/card", makeHTTPHandleFunc(s.handleUserCard))
	router.HandleFunc("/user/{id}/track/{key}/canStudy", makeHTTPHandleFunc(s.handleCanStudy))
	router.HandleFunc("/user/{id}/track/{key}/cardVerify", makeHTTPHandleFunc(s.handleUserCardVerify))
	router.HandleFunc("/user/{id}/track/{key}/card/{cardID}", makeHTTPHandleFunc(s.handleUserCardByID))
	router.HandleFunc("/user/{id}/track", makeHTTPHandleFunc(s.handleTrack))
	router.HandleFunc("/user/{id}/track/{key}", makeHTTPHandleFunc(s.handleTrackDelete))
	router.HandleFunc("/user/{id}/track/{key}/settings", makeHTTPHandleFunc(s.handleTrackSettingsByKey))
	// router.HandleFunc("/user/{id}/track/{key}/writing", makeHTTPHandleFunc(s.handleTrackSettingsByKey))
	// router.HandleFunc("/user/{id}/track/{key}/listening", makeHTTPHandleFunc(s.handleTrackSettingsByKey))
	// router.HandleFunc("/user/{id}/track/{key}/memory", makeHTTPHandleFunc(s.handleTrackSettingsByKey))
	router.HandleFunc("/user/{id}/track/{key}/test/{testName}", makeHTTPHandleFunc(s.handleTest))
	router.HandleFunc("/user/{id}/track/{key}/study/", makeHTTPHandleFunc(s.handleGetStudy))

	log.Println(("JSON API server running on port: "), s.listenAddr)

	handler := corsMiddleware(router)

	if err := http.ListenAndServe(s.listenAddr, handler); err != nil {
		log.Fatal("Server failed:", err)
	}

}

func (s *APIServer) cookiesAccepted(w http.ResponseWriter, r *http.Request) error {

	if r.Method == "POST" {

		var req = struct {
			Id int `json:"id"`
		}{}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return err
		}

		index := slices.IndexFunc(s.dataBase.Storage, func(user User) bool {
			return user.ID == req.Id
		})

		if index == -1 {
			return fmt.Errorf("ID is invalid")
		}

		s.dataBase.Storage[index].CokiesAccepted = true

		s.dataBase.UpdateData()
		return WriteJSON(w, http.StatusOK, struct {
			Token string `json:"token"`
		}{

			s.dataBase.Storage[index].Token,
		})
	} else {
		return fmt.Errorf("Method not allowed")
	}

}

func (s *APIServer) handleUserCardVerify(w http.ResponseWriter, r *http.Request) error {

	switch r.Method {
	case "POST":
		return s.handleVerifyCard(w, r)
	default:
		return fmt.Errorf("Method not allowed")
	}
}
func (s *APIServer) handleVerifyCard(w http.ResponseWriter, r *http.Request) error {
	req := new(newCardRequest)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	track, err := s.dataBase.GetTrack(r)
	if err != nil {
		return err
	}

	newCard := NewCard(track.DefineNewID(), req.Card.Data, req.Card.Notes, req.Card.TranslatedData, req.Card.Examples, req.Card.PronunciationPath)
	newCard.PronunciationPath = fmt.Sprintf("http://localhost:3000/audio?filename=%s.mp3", newCard.Data)

	card, err := track.VerifynewCard(newCard)

	var resp = NewCardResponse{Card: card, Added: true}
	resp.Card = card
	if err != nil {
		resp.Added = false
	}

	return WriteJSON(w, http.StatusOK, resp)
}

func (s *APIServer) handleVerifyUserName(w http.ResponseWriter, r *http.Request) error {

	if r.Method == "POST" {

		var req = new(VerifyUsernameReq)

		var response = struct {
			Allowed bool `json:"allowed"`
		}{}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return err
		}

		index := slices.IndexFunc(s.dataBase.Storage, func(user User) bool {
			return user.UserName == req.Username
		})

		if index != -1 {
			return WriteJSON(w, http.StatusOK, response)
		}

		response.Allowed = true
		return WriteJSON(w, http.StatusOK, response)
	} else {
		return fmt.Errorf("Method not allowed")
	}

}

func (s *APIServer) handleUser(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "GET":
		return s.handleGetUsers(w, r)
	default:
		return fmt.Errorf("Method not allowed")
	}
}

func (s *APIServer) handleRegister(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "PUT":
		return s.handleLogIn(w, r)
	case "POST":
		return s.handleSingUp(w, r)
	default:
		return fmt.Errorf("Method not allowed")
	}
}

func (s *APIServer) handleGetUsersToken(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "PUT":
		return s.GetUsersToken(r, w)
	default:
		return fmt.Errorf("Method not allowed")
	}
}

func (s *APIServer) handleGetUserByToken(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "PUT":
		return s.GetUserByToken(r, w)
	default:
		return fmt.Errorf("Method not allowed")
	}
}

func (s *APIServer) GetUserByToken(r *http.Request, w http.ResponseWriter) error {

	var token GetUserByTokenReq

	if err := json.NewDecoder(r.Body).Decode(&token); err != nil {
		return err
	}

	user, err := s.dataBase.GetUserByToken(token.Token)
	if err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, user)

}

func (s *APIServer) GetUsersToken(r *http.Request, w http.ResponseWriter) error {

	var req struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	user, err := s.dataBase.GetUserByID(req.ID)
	if err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, struct {
		Token string `json:"token"`
	}{user.Token})

}

type GetUserByTokenReq struct {
	Token string `json:"token"`
}

func (s *APIServer) handleUserCard(w http.ResponseWriter, r *http.Request) error {

	switch r.Method {
	case "GET":
		return s.handleGetCards(w, r)
	case "POST":
		return s.handlePostCard(w, r)
	default:
		return fmt.Errorf("Method not allowed")
	}
}

func (s *APIServer) handleGetCards(w http.ResponseWriter, r *http.Request) error {

	if r.Method == "GET" {

		card, err := s.dataBase.GetTrack(r)
		if err != nil {
			return err
		}

		copy := card.Copy().Storage
		slices.Reverse(copy)
		return WriteJSON(w, http.StatusOK, copy)
	} else {
		return fmt.Errorf("Undefined method")
	}
}

func (s *APIServer) handleLogIn(w http.ResponseWriter, r *http.Request) error {

	var req LogInReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	id, err := s.dataBase.LogIn(req)
	if err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, id)

}

func (s *APIServer) handleSingUp(w http.ResponseWriter, r *http.Request) error {

	var req SingUp

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	id, err := s.dataBase.SingUp(req)
	if err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, id)

}

func (s *APIServer) handlePostCard(w http.ResponseWriter, r *http.Request) error {
	req := new(newCardRequest)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	track, err := s.dataBase.GetTrack(r)
	if err != nil {
		return err
	}

	newCard := NewCard(track.DefineNewID(), req.Card.Data, req.Card.Notes, req.Card.TranslatedData, req.Card.Examples, req.Card.PronunciationPath)

	newCard.PronunciationPath = fmt.Sprintf("http://localhost:3000/audio?filename=%s.mp3", newCard.Data)

	card, err := track.AddNewCard(newCard, req.OldID)

	track.MissingTests()
	var resp = NewCardResponse{Card: card, Added: true}
	resp.Card = card
	if err != nil {
		resp.Added = false
	} else {
		s.dataBase.UpdateData()
	}

	return WriteJSON(w, http.StatusOK, card)
}

func (track *Track) AddNewCard(card Card, oldID int) (Card, error) {

	if oldID != -1 {
		track.Storage = slices.DeleteFunc(track.Storage, func(c Card) bool { return c.ID == oldID })

	}

	track.Storage = append(track.Storage, card)
	return card, nil
}
func (track *Track) VerifynewCard(card Card) (Card, error) {
	var index int

	if index = slices.IndexFunc(track.Storage, func(c Card) bool {
		return c.Data == card.Data
	}); index != -1 {
		return track.Storage[index], fmt.Errorf("Card already exists")
	}

	return card, nil
}

type NewCardResponse struct {
	Added bool `json:"allowed"`
	Card  Card `json:"card"`
}

func handleAudioRequest(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		http.Error(w, "Filename is required", http.StatusBadRequest)
		return
	}

	// Define the path to the audio files
	audioDir := "./audio/"
	audioPath := filepath.Join(audioDir, filename)

	// Check if the file exists
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Set the content type to audio/mpeg and serve the file
	w.Header().Set("Content-Type", "audio/mpeg")
	http.ServeFile(w, r, audioPath)
}

// func handleAudioRequest(w http.ResponseWriter, r *http.Request) {

// 	// Check if the file exists
// 	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
// 		http.Error(w, "File not found", http.StatusNotFound)
// 		return
// 	}

// 	// Set the content type and serve the file
// 	w.Header().Set("Content-Type", "audio/mpeg")
// 	http.ServeFile(w, r, audioPath)
// }

func (s *APIServer) handleUserCardByID(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "GET":
		return s.handleGetCardByID(w, r)
	case "POST":
		return s.handlePostCardByID(w, r)
	case "DELETE":
		return s.handleDeleteCardByID(w, r)
	default:
		return fmt.Errorf("Method not allowed")
	}
}

func (s *APIServer) handleGetCardByID(w http.ResponseWriter, r *http.Request) error {

	card, err := s.dataBase.GetCard(r)
	if err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, card)

}

func (s *APIServer) handleDeleteCardByID(w http.ResponseWriter, r *http.Request) error {

	card, err := s.dataBase.DeleteCard(r)
	if err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, card)

}

func (s *APIServer) handleGetNewCardData(w http.ResponseWriter, r *http.Request) error {

	if r.Method == "GET" {

		var fromLanguage, _ = getFromLanguage(r)
		var toLanguage, _ = getToLanguage(r)
		var expretion, _ = GetExpretion(r)

		card, err := GetNewCardData(fromLanguage, toLanguage, expretion, 3)

		if err != nil {

			return err
		}
		card.PronunciationPath = fmt.Sprintf("http://localhost:3000/audio?filename=%s.mp3", expretion)

		return WriteJSON(w, http.StatusOK, card)

	} else {
		return fmt.Errorf("Undefined method")
	}
}

func getFromLanguage(r *http.Request) (string, error) {

	fromLanguage := mux.Vars(r)["toLanguage"]

	return fromLanguage, nil
}

func getToLanguage(r *http.Request) (string, error) {

	toLanguage := mux.Vars(r)["toLanguage"]

	return toLanguage, nil
}
func GetExpretion(r *http.Request) (string, error) {
	expretion := mux.Vars(r)["expretion"]

	return expretion, nil
}

func (s *APIServer) handlePostCardByID(w http.ResponseWriter, r *http.Request) error {
	req := new(Card)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	card, err := s.dataBase.GetCard(r)
	if err != nil {
		return err
	}

	*card = req.Copy()
	go s.dataBase.UpdateData()

	return WriteJSON(w, http.StatusOK, card)

}

// Get /account
func (s *APIServer) handleGetUsers(w http.ResponseWriter, r *http.Request) error {

	return WriteJSON(w, http.StatusOK, s.dataBase.Storage)
}

func (s *APIServer) handeUser(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "GET":
		return s.handleGetUserByID(w, r)
	case "DELETE":
		return s.handleDeleteUserByID(w, r)
	default:
		return fmt.Errorf("Method undefiend")
	}
}

func (s *APIServer) handleGetUserByID(w http.ResponseWriter, r *http.Request) error {

	account, err := s.dataBase.GetUser(r)
	if err != nil {
		return err
	}

	user := account.Copy()
	user.Tracks = nil

	return WriteJSON(w, http.StatusOK, user)

}

func (s *APIServer) handleDeleteUserByID(w http.ResponseWriter, r *http.Request) error {

	account, err := s.dataBase.GetUser(r)
	if err != nil {
		return err
	}

	s.dataBase.Storage = slices.DeleteFunc(s.dataBase.Storage, func(user User) bool {
		return user.ID == account.ID
	})

	s.dataBase.UpdateData()

	return WriteJSON(w, http.StatusOK, struct {
		Status string `json:"status"`
	}{
		Status: "deleted",
	})
}

func (s *APIServer) handleGetTrackStorageByKey(w http.ResponseWriter, r *http.Request) error {

	track, err := s.dataBase.GetTrack(r)
	if err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, track.Storage)

}

func (s *APIServer) handleTrackStorageByKey(w http.ResponseWriter, r *http.Request) error {

	switch r.Method {
	case "GET":
		return s.handleTrackStorageByKey(w, r)
	case "POST":
		return s.handlePostCard(w, r)
	default:
		return fmt.Errorf("Method not allowed")
	}

}

func (s *APIServer) handleGetTest(w http.ResponseWriter, r *http.Request) error {

	track, err := s.dataBase.GetTrack(r)
	if err != nil {
		return err
	}

	test, err := track.GetTest(r)

	if err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, StudyAnswer{Cards: test, FakeAnswers: track.getFakeAnswers()})

}

func (s *APIServer) handlePostTest(w http.ResponseWriter, r *http.Request) error {

	statusRequest := new(CreateTestStatusRequest)

	if err := json.NewDecoder(r.Body).Decode(&statusRequest); err != nil {
		return err
	}

	track, err := s.dataBase.GetTrack(r)
	if err != nil {
		return err
	}

	name, _ := getTestName(r)
	test, err := track.defineTest(r)

	if err != nil {
		return err
	}

	err = test.DefineStatusUpdate(statusRequest, track.Settings.DaylyTestTries)
	if err != nil {
		return err
	}

	err = track.updateTestDates(name, statusRequest.IDs, test.Status)
	if err != nil {
		return err
	}
	s.dataBase.UpdateData()
	store, _ := OpenStorage("./storage.json")
	s.dataBase = store

	return WriteJSON(w, http.StatusOK, TestResponse{test.Status, test.DaylyTestTries, fmt.Sprintf("You have %v tries left. Study) \nTest status: %v", test.DaylyTestTries, test.Status)})

}

func (t *Track) updateTestDates(testName string, IDs []int, testStatus string) error {

	for i, _ := range t.Storage {
		card := &t.Storage[i]
		if slices.Contains(IDs, card.ID) {
			test, err := card.getTest(testName)
			if err != nil {
				return err
			}
			if testStatus == "passed" {
				test.Repeated++
				test.defineRepeatDate()

			} else if testStatus == "failed" {
				test.Repeated = 0
				test.defineRepeatDate()
			}
		}
	}
	return nil
}

func (t *TestData) defineRepeatDate() {

	date := time.Now()

	switch t.Repeated {
	case 0:
		date = date.AddDate(0, 0, 1)
	case 1:
		date = date.AddDate(0, 0, 1)
	case 2:
		date = date.AddDate(0, 0, 1)
	case 3:
		date = date.AddDate(0, 0, 3)
	case 4:
		date = date.AddDate(0, 0, 5)
	case 5:
		date = date.AddDate(0, 0, 7)
	case 6:
		date = date.AddDate(0, 0, 14)
	case 7:
		date = date.AddDate(0, 0, 30)
	case 8:
		date = date.AddDate(0, 0, 60)
	case 9:
		date = date.AddDate(0, 0, 240)
	}

	t.ReapeatDate = date.Format("2006.01.02")
}

func (c *Card) getTest(testName string) (*TestData, error) {
	switch testName {
	case "listening":
		return &c.Listening, nil
	case "fromLanguage":
		return &c.FromLanguage, nil
	case "toLanguage":
		return &c.ToLanguage, nil
	case "writing":
		return &c.Writing, nil
	}

	return nil, fmt.Errorf("Undefined tast name: %v", testName)
}

type TestResponse struct {
	Status         string `json:"status"`
	DaylyTestTries int    `json:"daylyTestTries"`
	Message        string `json:"message"`
}

func (s *APIServer) handleTest(w http.ResponseWriter, r *http.Request) error {

	switch r.Method {
	case "POST":
		return s.handlePostTest(w, r)
	case "GET":
		return s.handleGetTest(w, r)
	default:
		return fmt.Errorf("Undefined method")
	}

}

func (s *APIServer) handleGetStudy(w http.ResponseWriter, r *http.Request) error {

	track, err := s.dataBase.GetTrack(r)
	if err != nil {
		return err
	}

	cardsToStudy, err := track.GetStudy()
	if err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, cardsToStudy)

}

func (s *APIServer) handleCanStudy(w http.ResponseWriter, r *http.Request) error {

	track, err := s.dataBase.GetTrack(r)
	if err != nil {
		return err
	}

	cardsToStudy, err := track.GetStudy()
	if err != nil {
		return err
	}

	var canStudy = false

	if len(cardsToStudy.Cards) >= 3 {
		canStudy = true
	}
	return WriteJSON(w, http.StatusOK, struct {
		CanStudy bool `json:"canStudy"`
	}{
		CanStudy: canStudy,
	})

}

func (s *APIServer) handleGetTrackSettingsByKey(w http.ResponseWriter, r *http.Request) error {

	track, err := s.dataBase.GetTrack(r)
	if err != nil {
		return err
	}

	track.MissingTests()
	return WriteJSON(w, http.StatusOK, GetTrackSettings{track.Settings, TestsStatuses{
		track.Listening, track.Writing, track.ToLanguage, track.FromLanguage,
	}})

}

type GetTrackSettings struct {
	Settings      TrackSettings `json:"settings"`
	TestsStatuses TestsStatuses `json:"testsStatuses"`
}

type TestsStatuses struct {
	Listening    Test `json:"listening"`
	Writing      Test `json:"writing"`
	ToLanguage   Test `json:"fromLanguage"`
	FromLanguage Test `json:"toLanguage"`
}

func (s *APIServer) handlePostTrackSettingsByKey(w http.ResponseWriter, r *http.Request) error {
	createTrackSettingsReq := new(TrackSettings)

	if err := json.NewDecoder(r.Body).Decode(&createTrackSettingsReq); err != nil {
		return err
	}

	track, err := s.dataBase.GetTrack(r)
	if err != nil {
		return err
	}

	track.Settings = *createTrackSettingsReq
	go s.dataBase.UpdateData()

	trackCopy := track.Copy()
	trackCopy.Storage = nil
	return WriteJSON(w, http.StatusOK, trackCopy)

}

func (s *APIServer) handleDeleteTrackByKey(w http.ResponseWriter, r *http.Request) error {

	user, err := s.dataBase.GetUser(r)
	if err != nil {
		return err
	}

	key, err := GetKey(r)
	if err != nil {
		return err
	}

	user.Tracks = slices.DeleteFunc(user.Tracks, func(t Track) bool { return key == t.Name })

	user.TracksKeys = slices.DeleteFunc(user.TracksKeys, func(k string) bool {

		return key == k
	})

	go s.dataBase.UpdateData()

	return WriteJSON(w, http.StatusOK, key)

}

func (s *APIServer) handleTrackSettingsByKey(w http.ResponseWriter, r *http.Request) error {

	switch r.Method {
	case "POST":
		return s.handlePostTrackSettingsByKey(w, r)
	case "GET":
		return s.handleGetTrackSettingsByKey(w, r)
	default:
		return fmt.Errorf("Undefined method")
	}

}

func (s *APIServer) handleTrackDelete(w http.ResponseWriter, r *http.Request) error {

	switch r.Method {
	case "DELETE":
		return s.handleDeleteTrackByKey(w, r)

	default:
		return fmt.Errorf("Undefined method")
	}

}

func (s *APIServer) handleTrack(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case "GET":
		return s.handleGetTracks(w, r)
	case "POST":
		return s.handleCreateTrack(w, r)
	default:
		return fmt.Errorf("Method not allowed")
	}
}

func (s *APIServer) handleCreateTrack(w http.ResponseWriter, r *http.Request) error {
	createTrackReq := new(CreateTrackRequest)

	if err := json.NewDecoder(r.Body).Decode(&createTrackReq); err != nil {
		return err
	}

	track := NewTrack(createTrackReq)

	err := s.dataBase.CreateTrack(track, r)

	if err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, track)
}

func (s *APIServer) handleGetTracks(w http.ResponseWriter, r *http.Request) error {

	account, err := s.dataBase.GetUser(r)
	if err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, account.Tracks)
}
