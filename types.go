package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type User struct {
	ID             int      `json:"id"`
	FirstName      string   `json:"firstName"`
	Token          string   `json:"token"`
	CokiesAccepted bool     `json:"cokiesAccepted"`
	LastName       string   `json:"lastName"`
	EMail          string   `json:"eMail"`
	Tracks         []Track  `json:"tracks"`
	TracksKeys     []string `json:"tracksKeys"`
	Settings       Settings `json:"settings"`
	UserName       string   `json:"userName"`
	Password       string   `json:"password"`
}

func (u User) Copy() User {
	return u
}

type VerifyUsernameReq struct {
	Username string `json:"username"`
}

func NewUser(firstName, lastName, eMail, userName, password string) *User {

	return &User{

		FirstName: firstName,
		Token:     userName,
		LastName:  lastName,
		EMail:     eMail,
		UserName:  userName,
		Password:  password,
		Tracks:    []Track{},
	}
}

func NewTrack(req *CreateTrackRequest) Track {
	var track = Track{
		Name: req.Name,

		ToLanguage:   Test{Name: req.ToLanguage, DaylyTestTries: req.DaylyTestTries, Status: "missing"},
		FromLanguage: Test{Name: req.FromLanguage, DaylyTestTries: req.DaylyTestTries, Status: "missing"},
		Writing:      Test{Name: "writing", DaylyTestTries: req.DaylyTestTries, Status: "missing"},
		Listening:    Test{Name: "listening", DaylyTestTries: req.DaylyTestTries, Status: "missing"},
		Settings: TrackSettings{
			Name:              req.Name,
			SumUnstudiedCards: req.SumUnstudiedCards,
			SumUntestedCards:  req.SumUntestedCards,

			UseExamples:     req.UseExamples,
			UseNotes:        req.UseNotes,
			DaylyTestTries:  req.DaylyTestTries,
			DaylyTestCards:  req.DaylyStudyCards,
			DaylyStudyCards: req.DaylyTestCards,
		},
		Storage: []Card{},
	}

	track.Name = track.FromLanguage.Name + "-" + track.ToLanguage.Name
	track.Settings.Name = track.FromLanguage.Name + "-" + track.ToLanguage.Name

	if req.Writing {
		track.Settings.Writing = true
	}

	if req.Listening {
		track.Settings.Listening = true
	}

	return track
}

type CreateAccountRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	EMail     string `json:"eMail"`
	UserName  string `json:"userName"`
	Password  string `json:"password"`
}

type CreateTrackRequest struct {
	Name              string `json:"name"`
	FromLanguage      string `json:"fromLanguage"`
	ToLanguage        string `json:"toLanguage"`
	UseExamples       bool   `json:"useExamples"`
	UseNotes          bool   `json:"useNotes"`
	Listening         bool   `json:"listening"`
	Writing           bool   `json:"writing"`
	DaylyTestTries    int    `json:"daylyTestTries"`
	SumUnstudiedCards bool   `json:"sumUnstudiedCards"`

	SumUntestedCards bool `json:"sumUntestedCards"`

	DaylyTestCards  int `json:"daylyTestCards"`
	DaylyStudyCards int `json:"daylyStudyCards"`
}

type CreateTestStatusRequest struct {
	Passed bool  `json:"passed"`
	IDs    []int `json:"IDs"`
}

type Settings struct {
	ReminderStatus bool   `json:"reminderStatus"`
	ReminderDate   string `json:"reminderDate"`
	DarkTheme      bool   `json:"darkTheme"`
}

type Track struct {
	Name         string        `json:"name"`
	Storage      []Card        `json:"storage"`
	FromLanguage Test          `json:"fromLanguage"`
	ToLanguage   Test          `json:"toLanguage"`
	Listening    Test          `json:"listening"`
	Writing      Test          `json:"writing"`
	Settings     TrackSettings `json:"settings"`
}

func (t Track) GetTest(r *http.Request) ([]Card, error) {
	var name, err = getTestName(r)
	if err != nil {
		return []Card{}, err
	}

	test, err := t.defineTest(r)
	if err != nil {
		return []Card{}, err
	}

	if test.Status == "failed" || test.Status == "passed" {
		return []Card{}, t.defineTestError(test)
	}

	max := t.Settings.getMaxTestCards()
	var cards = make([]Card, 0, max)
	var i int
	var todaysDate = time.Now().Format("2006.01.02")
	for _, card := range t.Storage {
		switch name {
		case "listening":
			if card.Listening.ReapeatDate == todaysDate {
				cards = append(cards, card)
				i++
			}
		case "toLanguage":
			if card.ToLanguage.ReapeatDate == todaysDate {
				cards = append(cards, card)
				i++
			}

		case "fromLanguage":
			if card.FromLanguage.ReapeatDate == todaysDate {
				cards = append(cards, card)
				i++
			}

		case "writing":
			if card.Writing.ReapeatDate == todaysDate {
				cards = append(cards, card)
				i++
			}
		}
		if i == max {
			break
		}
	}

	if i == 0 {
		return []Card{}, t.defineTestError(test)
	}
	return cards, nil
}

func (t *Track) GetTestData(r *http.Request) (*Test, error) {
	test, err := t.defineTest(r)
	if err != nil {
		return nil, err
	}

	return test, nil
}

func getTestName(r *http.Request) (string, error) {
	idStr := mux.Vars(r)["testName"]

	return idStr, nil

}

type StudyAnswer struct {
	Cards       []Card `json:"cards"`
	FakeAnswers []Card `json:"fakeAnswers"`
}

func (t Track) GetStudy() (StudyAnswer, error) {

	var maxCards = t.Settings.getMaxStudyCards()
	var cards = make([]Card, 0, maxCards)
	var failedTestCards = make([]Card, 0, maxCards)
	var todaysDate = time.Now().Format("2006.01.02")
	var fakeAnswers = make([]Card, 0, maxCards*2)

	for _, card := range t.Storage {

		if verifyCard(card, todaysDate) {

			var cardsLength, failedCardsLength = len(cards), len(failedTestCards)
			if card.CreationDate != todaysDate && failedCardsLength < maxCards {
				failedTestCards = append(failedTestCards)
			} else if cardsLength < maxCards {
				cards = append(cards, card)
			} else {
				fakeAnswers = append(fakeAnswers, card)
			}

			if cardsLength == maxCards && failedCardsLength == maxCards {
				break
			}
		}

	}

	var result = []Card{}
	if t.Settings.FailedTestCardsPriopity {
		result = append(failedTestCards, cards...)
	} else {
		result = append(cards, failedTestCards...)
	}

	if len(result) == 0 {
		return StudyAnswer{}, fmt.Errorf("There are no cards to study")
	}

	return StudyAnswer{Cards: result, FakeAnswers: t.getFakeAnswers()}, nil
}

func (t Track) getFakeAnswers() []Card {
	// Create a new slice with the same length as t.Storage
	var cards = make([]Card, len(t.Storage))

	// Iterate over the t.Storage slice and deep copy each Card
	for i, originalCard := range t.Storage {
		// Copy the card itself
		cards[i] = originalCard

		// Deep copy the TranslatedData slice to ensure it's a new slice
		copiedTranslatedData := make([]string, len(originalCard.TranslatedData))
		copy(copiedTranslatedData, originalCard.TranslatedData)

		// Assign the deep-copied TranslatedData back to the copied card
		cards[i].TranslatedData = copiedTranslatedData

		// Modify the copied card (not the original)
		cards[i].Data += " Fake"
		cards[i].TranslatedData[len(cards[i].TranslatedData)-1] += " Fake"
	}

	return cards
}

func verifyCard(card Card, todaysDate string) bool {

	switch {
	case card.FromLanguage.ReapeatDate == todaysDate:
		return true
	case card.ToLanguage.ReapeatDate == todaysDate:
		return true
	case card.Writing.ReapeatDate == todaysDate:
		return true
	case card.Listening.ReapeatDate == todaysDate:
		return true
	default:
		return false
	}
}

func (t *Track) defineTest(r *http.Request) (*Test, error) {
	name1, err := getTestName(r)
	if err != nil {
		return nil, err
	}

	var test *Test
	var testUsed error = nil
	switch name1 {
	case "listening":
		test = &t.Listening
		if !t.Settings.Listening {

			testUsed = fmt.Errorf("Listenign test isn't used by the user")
		}
	case "toLanguage":
		test = &t.ToLanguage

	case "fromLanguage":
		test = &t.FromLanguage

	case "writing":
		test = &t.Writing
		if !t.Settings.Writing {

			testUsed = fmt.Errorf("Writing test isn't used by the user")
		}
	default:
		return nil, fmt.Errorf("Undefined test type: %v", name1)
	}

	return test, testUsed
}

func (t Track) defineTestError(test *Test) error {

	switch test.Status {
	case "prepared":
		return fmt.Errorf("There are no cards to test today")
	case "failed":
		return fmt.Errorf("Test is failed, try tomorow")
	case "passed":
		return fmt.Errorf("Test has been passed")

	}
	return fmt.Errorf("defineTest error")
}

func (s TrackSettings) getMaxTestCards() int {
	if s.DaylyTestCards == -1 {
		return 0
	}
	return s.DaylyTestCards
}

func (s TrackSettings) getMaxStudyCards() int {
	if s.DaylyStudyCards == -1 {
		return 0
	}
	return s.DaylyStudyCards
}

func (t Track) Copy() Track {
	return t
}

func (t Track) DefineNewID() int {
	var id int = 0

	if len(t.Storage) == 0 {
		id = 1
	} else {
		id = t.Storage[len(t.Storage)-1].ID + 1
	}

	return id
}

func (t *Track) MissingTests() {
	todaysDate := time.Now().Format("2006.01.02")
	var fromLg int
	var toLg int
	var listening int
	var writing int
	for _, card := range t.Storage {
		if card.FromLanguage.ReapeatDate == todaysDate {
			fromLg++
		}
		if card.ToLanguage.ReapeatDate == todaysDate {
			toLg++
		}
		if card.Listening.ReapeatDate == todaysDate {
			listening++
		}
		if card.Writing.ReapeatDate == todaysDate {
			writing++
		}
	}

	if fromLg >= t.Settings.DaylyTestCards {
		t.FromLanguage.Status = "prepared"
	} else {
		t.FromLanguage.Status = "missing"
	}

	if toLg >= t.Settings.DaylyTestCards {
		t.ToLanguage.Status = "prepared"
	} else {
		t.ToLanguage.Status = "missing"
	}

	if listening >= t.Settings.DaylyTestCards {
		t.Listening.Status = "prepared"
	} else {
		t.Listening.Status = "missing"
	}

	if writing >= t.Settings.DaylyTestCards {
		t.Writing.Status = "prepared"
	} else {
		t.Writing.Status = "missing"
	}
}

type TrackSettings struct {
	Name              string `json:"name"`
	SumUnstudiedCards bool   `json:"sumUnstudiedCards"`

	SumUntestedCards        bool `json:"sumUntesteddCards"`
	FailedTestCardsPriopity bool `json:"failedTestCardsPriopity"` //Implement procces!!!!!!
	UseExamples             bool `json:"useExamples"`
	UseNotes                bool `json:"useNotes"`
	Writing                 bool `json:"writing"`
	Listening               bool `json:"listening"`
	DaylyTestTries          int  `json:"daylyTestTries"`
	DaylyTestCards          int  `json:"daylyTestCards"`
	DaylyStudyCards         int  `json:"daylyStudyCards"`
}

type Test struct {
	Name           string `json:"name"`
	DaylyTestTries int    `json:"daylyTestTries"`
	LastFailDate   string `json:"lastFailDate"`
	LastPassedDate string `json:"lastPassedDate"`
	Status         string `json:"status"`
}

func (test *Test) VerifyTestStatuses(maxTestTries int) {
	todaysDate, _ := time.Parse("2006.01.02", time.Now().Format("2006.01.02"))

	switch test.Status {
	case "prepared":
		test.Status = "missing"
	case "passsed":
		var date, _ = time.Parse("2006.01.02", test.LastPassedDate)
		if date.Before(todaysDate) {
			test.DaylyTestTries = maxTestTries
			test.Status = "missing"
		}
	case "tried":
	case "failed":
		var date, _ = time.Parse("2006.01.02", test.LastFailDate)
		if date.Before(todaysDate) {
			test.DaylyTestTries = maxTestTries
			test.Status = "missing"
		}
	}

}

func (t Test) Copy() Test {
	return t
}

func (t *Test) DefineStatusUpdate(req *CreateTestStatusRequest, maxTestTries int) error {
	var todaysDate = time.Now().Format("2006.01.02")
	if req.Passed {

		t.Status = "passed"
		t.LastPassedDate = todaysDate
		t.DaylyTestTries = maxTestTries
	} else {
		if t.DaylyTestTries == 0 {
			return fmt.Errorf("Test is failed")
		}
		t.DaylyTestTries--
		if t.DaylyTestTries == 0 {
			t.LastFailDate = todaysDate
			t.Status = "failed"
		} else {
			t.Status = "tried"
		}
	}

	return nil

}

type Card struct {
	ID                int      `json:"id"`
	Data              string   `json:"name"`
	TranslatedData    []string `json:"translations"`
	Examples          []string `json:"examples"`
	Notes             string   `json:"notes"`
	FromLanguage      TestData `json:"fromLanguage"`
	ToLanguage        TestData `json:"toLanguage"`
	Listening         TestData `json:"listening"`
	Writing           TestData `json:"writing"`
	CreationDate      string   `json:"creationDate"`
	PronunciationPath string   `json:"pronunciation"`
}

func (test *TestData) VerifyDates() {
	todaysDate, _ := time.Parse("2006.01.02", time.Now().Format("2006.01.02"))
	repeatDate, _ := time.Parse("2006.01.02", test.ReapeatDate)

	formatedTodaysDate := todaysDate.Format("2006.01.02")
	if repeatDate.Before(todaysDate) {
		test.ReapeatDate = formatedTodaysDate

	}

}

type CardData struct {
	Name              string        `json:"name"`
	Translations      []Translation `json:"translations"`
	PronunciationPath string        `json:"pronunciationPath"`
}

type Translation struct {
	Translation string   `json:"translation"`
	Examples    []string `json:"examples"`
}

type newCardRequest struct {
	Card  F   `json:"card"`
	OldID int `json:"oldID"`
}

type F struct {
	Data              string   `json:"name"`
	TranslatedData    []string `json:"translations"`
	Examples          []string `json:"examples"`
	Notes             string   `json:"notes"`
	PronunciationPath string   `json:"pronunciationPath"`
}

// export interface Card {
// 	name: string

// 	translations: string[]
// 	examples: string[]
// 	notes: string

// 	pronunciationPath: string
//   }

func NewCard(id int, data, notes string, translatedData, examples []string, pronunciationPath string) Card {
	var todaysDate = time.Now().Format("2006.01.02")
	return Card{
		ID:                id,
		Data:              data,
		TranslatedData:    translatedData,
		Examples:          examples,
		Notes:             notes,
		FromLanguage:      TestData{ReapeatDate: todaysDate},
		ToLanguage:        TestData{ReapeatDate: todaysDate},
		Listening:         TestData{ReapeatDate: todaysDate},
		Writing:           TestData{ReapeatDate: todaysDate},
		CreationDate:      todaysDate,
		PronunciationPath: pronunciationPath,
	}
}

func (c Card) Copy() Card {
	return c
}

type TestData struct {
	TestQuize   bool   `json:"testQuize"`
	ReapeatDate string `json:"repeatDate"`
	Repeated    int    `json:"repeated"`
}

type LogInReq struct {
	UsernameEMail string `json:"userNameEMail"`
	Password      string `json:"password"`
}

type SingUp struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Username  string `json:"userName"`
	EMail     string `json:"eMail"`
	Password  string `json:"password"`
}
