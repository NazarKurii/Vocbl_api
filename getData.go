package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

var (
	ExprationDataNotFound = errors.New("Expration data is not found.")
	PronunciationNotFound = errors.New("Pronunciation data is not found.")
	NoTranslationFound    = errors.New("Translation data is not found.")
)

const (
	ExampleApiKey     = "8b2ebc45-6b7c-4575-89be-537ca676f94a"
	TranslationApiKey = "53e30d15-e741-4a64-bb35-43ae0c36745e:fx"
	RequestAttemts    = 3
)

type ExpretionData struct {
	Translations  []Translation
	Pronunciation Pronunciation
}

type Response struct {
	Hwi struct {
		Prs []struct {
			Mw    string `json:"mw"`
			Sound struct {
				Audio string `json:"audio"`
			} `json:"sound"`
		} `json:"prs"`
	} `json:"hwi"`

	Def []struct {
		Sseq [][][]interface{} `json:"sseq"`
	} `json:"def"`
}

type DeepLResponse struct {
	Translations []struct {
		DetectedSourceLanguage string `json:"detected_source_language"`
		Text                   string `json:"text"`
	} `json:"translations"`
}

func GetNewCardData(fromLanguage, toLanguage, expretion string, requestAttemts int) (CardData, error) {
	url := fmt.Sprintf("https://www.dictionaryapi.com/api/v3/references/learners/json/%s?key=%s", expretion, ExampleApiKey)

	resp, err := http.Get(url)
	if err != nil {

		if requestAttemts != 0 {
			return GetNewCardData(fromLanguage, toLanguage, expretion, requestAttemts-1)
		} else {
			return CardData{}, ExprationDataNotFound
		}
	}
	defer resp.Body.Close()

	// Decode the JSON response
	var data []Response
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		fmt.Println(err)
		return CardData{}, ExprationDataNotFound
	}

	// Check if data is empty
	if len(data) == 0 {
		return CardData{}, ExprationDataNotFound
	}

	var examples []string

	for _, def := range data[0].Def {
		for _, sseq := range def.Sseq {
			for _, sense := range sseq {
				if len(sense) > 1 {
					senseData, ok := sense[1].(map[string]interface{})
					if !ok {
						continue
					}
					if dt, ok := senseData["dt"].([]interface{}); ok {
						for _, item := range dt {
							itemData, ok := item.([]interface{})
							if !ok || len(itemData) < 2 {
								continue
							}
							if itemData[0] == "vis" {
								for _, vis := range itemData[1].([]interface{}) {
									visData, ok := vis.(map[string]interface{})
									if !ok {
										continue
									}
									if example, ok := visData["t"].(string); ok {
										examples = append(examples, example)
									}
								}
							}
						}
					}
				}
			}
		}
	}

	examples = correctExamples(examples)

	var translations []Translation

	if translation, err := getTransltion(expretion, "", fromLanguage, toLanguage, requestAttemts); err == nil {
		translations = append(translations, Translation{Translation: translation})
	}

	for _, context := range examples {
		contextTranslation, err := getTransltion(expretion, context, fromLanguage, toLanguage, requestAttemts)

		if err != nil {
			continue
		}
		existingContextTranslationIndex := slices.IndexFunc(translations, func(translation Translation) bool {
			return translation.Translation == contextTranslation
		})

		if existingContextTranslationIndex == -1 {
			translations = append(translations, Translation{contextTranslation, []string{context}})
		} else {
			translations[existingContextTranslationIndex].Examples = append(translations[existingContextTranslationIndex].Examples, context)
		}
	}
	translations = sortTranslations(translations)

	translations = filterVerbs(translations)

	path := getPronuciation(expretion, data)

	var CardData = CardData{Translations: translations, PronunciationPath: path.Path}

	return CardData, nil
}

func sortTranslations(translations []Translation) []Translation {

	var sortTranslations = make([]Translation, len(translations))

	for i, translation := range translations {
		var newTranslation = Translation{Examples: translation.Examples}
		var newName []rune

		for _, character := range translation.Translation {

			if unicode.IsLetter(character) || unicode.IsSpace(character) {

				newName = append(newName, character)
			}
		}
		newName[0] = unicode.ToUpper(newName[0])
		newTranslation.Translation = string(newName)
		sortTranslations[i] = newTranslation
	}
	return sortTranslations
}
func filterVerbs(translations []Translation) []Translation {
	var infinitives []Translation
	for _, translation := range translations {

		if strings.ToLower(translation.Translation)[len(translation.Translation)-1] == 184 && func() bool {
			for i, infinitive := range infinitives {

				if strings.Compare(infinitive.Translation[:2], translation.Translation) == 0 {
					if translation.Translation[0] >= 65 && translation.Translation[0] <= 122 {
						return true
					}
					infinitives[i].Examples = append(infinitive.Examples, translation.Examples...)
					return true

				}

			}
			return false
		}() {

			infinitives = append(infinitives, translation)
		} else {
			var new = true
			translationToCompare := translation.Translation[:2]
			for i, infinitive := range infinitives {

				if strings.Compare(infinitive.Translation[:2], translationToCompare) == 0 {
					infinitives[i].Examples = append(infinitive.Examples, translation.Examples...)
					new = false
					break
				}
			}
			if new {
				if translation.Translation[0] >= 65 && translation.Translation[0] <= 122 {

				} else {
					infinitives = append(infinitives, translation)

				}
			}
		}
	}

	if len(infinitives) == 0 {
		return translations
	}
	return infinitives

}

type Pronunciation struct {
	Phonetic string
	Path     string
}

type Sound struct {
	Audio string `json:"audio"`
}

type Pr struct {
	Mw    string `json:"mw"`
	Sound Sound  `json:"sound"`
}

type Hwi struct {
	Prs []Pr `json:"prs"`
}

type Responsed struct {
	Hwi Hwi `json:"hwi"`
}

func getPronuciation(expretion string, data []Response) Pronunciation {
	var pronunciation Pronunciation
	for _, pr := range data[0].Hwi.Prs {
		if pr.Mw != "" {
			pronunciation.Phonetic = pr.Mw
		}
		if pr.Sound.Audio != "" {
			// Determine the subdirectory
			audio := pr.Sound.Audio
			var subdirectory string
			if strings.HasPrefix(audio, "bix") {
				subdirectory = "bix"
			} else if strings.HasPrefix(audio, "gg") {
				subdirectory = "gg"
			} else if audio[0] >= '0' && audio[0] <= '9' || audio[0] == '_' {
				subdirectory = "number"
			} else {
				subdirectory = string(audio[0])
			}

			// Construct the URL
			audioURL := fmt.Sprintf("https://media.merriam-webster.com/audio/prons/en/us/mp3/%s/%s.mp3", subdirectory, audio)
			err := downloadFile(expretion, audioURL)
			if err == nil {
				pronunciation.Path = fmt.Sprintf("../audio/%v.mp3", strings.ToLower(expretion))
			} else {
				fmt.Println("Error downloading file:", err)
			}
		}
	}

	return pronunciation
}

func downloadFile(expretion string, url string) error {
	// Create the file
	filePath := fmt.Sprintf("./audio/%v.mp3", strings.ToLower(expretion))
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	// Check if the file is empty
	fileInfo, err := out.Stat()
	if err != nil {
		return err
	}
	if fileInfo.Size() == 0 {
		return fmt.Errorf("downloaded file is empty: %v", filePath)
	}

	return nil
}

func correctExamples(examples []string) []string {
	var corrextedExamples = make([]string, len(examples))
	re := regexp.MustCompile(`\[.*?\]|\{[^}]*\}`)
	for i, example := range examples {
		corrextedExamples[i] = re.ReplaceAllString(example, "")
	}
	return corrextedExamples
}

var languages = map[string]string{
	"English":   "EN",
	"German":    "DE",
	"Spanish":   "ES",
	"French":    "FR",
	"Italian":   "IT",
	"Polish":    "PL",
	"Swedish":   "SV",
	"Ukrainian": "UK",
}

func getTransltion(expretion, context, fromLanguage, toLanguage string, tries int) (string, error) {

	apiURL := "https://api-free.deepl.com/v2/translate"

	data := url.Values{}
	data.Set("auth_key", TranslationApiKey)
	data.Set("text", expretion)

	data.Set("source_lang", languages[toLanguage])
	data.Set("target_lang", languages[fromLanguage])
	// data.Set("source_lang", "En")
	// data.Set("target_lang", "Uk")
	data.Set("context", context)

	resp, err := http.PostForm(apiURL, data)
	if err != nil {

		if tries != 0 {
			return getTransltion(expretion, context, fromLanguage, toLanguage, tries-1)
		} else {

			return "", NoTranslationFound
		}
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	var deepLResponse DeepLResponse

	if err := json.Unmarshal(bodyBytes, &deepLResponse); err != nil {
		fmt.Println("Error decoding the response:", err)

	}

	if len(deepLResponse.Translations) > 0 {

		return deepLResponse.Translations[0].Text, nil
	} else {
		return deepLResponse.Translations[0].Text, NoTranslationFound
	}

}
