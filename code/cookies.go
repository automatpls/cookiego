package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type CookieEntry struct {
	URL     string            `json:"url"`
	Cookies map[string]string `json:"cookies"`
}

func getCookieString(cookieMap map[string]string) []string {
	var result []string
	for k, v := range cookieMap {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

func SaveCookiesToFile(link string, cookies []*http.Cookie) CookieEntry {
	cookieMap := make(map[string]string)
	var cookieStr []string
	for _, cookie := range cookies {
		cookieStr = append(cookieStr, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
		cookieMap[cookie.Name] = cookie.Value
	}

	savedLinks := loadSavedCookieLinks()
	if savedLinks[link] {
		LogEvent(fmt.Sprintf("Запись для %s, уже существует", link))
		return CookieEntry{URL: link, Cookies: cookieMap}
	}

	line := fmt.Sprintf("%s %s\n", link, strings.Join(cookieStr, "; "))
	cookiesFile, err := os.OpenFile("OUTPUT/cookies.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer cookiesFile.Close()
		cookiesFile.WriteString(line)
	}
	return CookieEntry{
		URL:     link,
		Cookies: cookieMap,
	}
}
func loadSavedCookieLinks() map[string]bool {
	savedLinks := make(map[string]bool)

	file, err := os.Open("OUTPUT/cookies.txt")
	if err != nil {
		return savedLinks
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) > 0 {
			savedLinks[parts[0]] = true
		}
	}
	return savedLinks
}
func loadSavedJSONLinks() map[string]bool {
	saved := make(map[string]bool)

	file, err := os.Open("OUTPUT/cookies.json")
	if err != nil {
		return saved
	}
	defer file.Close()

	var entries []CookieEntry
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&entries); err != nil {
		return saved
	}

	for _, entry := range entries {
		saved[entry.URL] = true
	}
	return saved
}
func WriteCookiesJSON(entries []CookieEntry) error {
	filePath := "OUTPUT/cookies.json"
	var existing []CookieEntry
	if _, err := os.Stat(filePath); err == nil {

		file, err := os.Open(filePath)
		if err != nil {
			LogEvent(fmt.Sprintf("Ошибка при открытии cookies.json: %v", err))
			return fmt.Errorf("не удалось открыть cookies.json: %v", err)
		}
		defer file.Close()

		err = json.NewDecoder(file).Decode(&existing)
		if err != nil {
			LogEvent(fmt.Sprintf("Ошибка при декодировании cookies.json: %v", err))
			return fmt.Errorf("не удалось декодировать cookies.json: %v", err)
		}
	}
	for _, entry := range entries {
		duplicate := false
		for _, existingEntry := range existing {
			if entry.URL == existingEntry.URL {
				duplicate = true
				break
			}
		}
		if duplicate {
			LogEvent(fmt.Sprintf("Кука для URL %s уже существует, пропускаем.", entry.URL))
			continue
		}

		existing = append(existing, entry)
	}

	outFile, err := os.Create(filePath)
	if err != nil {
		LogEvent(fmt.Sprintf("Ошибка создания cookies.json: %v", err))
		return fmt.Errorf("не удалось создать cookies.json: %v", err)
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")
	LogEvent("Куки сохранены в cookies.json")
	return encoder.Encode(existing)
}
