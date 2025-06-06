package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

func CheckConnection(link string, client *http.Client, wg *sync.WaitGroup, connResults chan<- string, cookieResults chan<- string, jsonEntries chan<- CookieEntry, reportEntries chan<- HTMLReportEntry) {
	defer wg.Done()

	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:79.0) Gecko/20100101 Firefox/79.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.1 Safari/605.1.15",
	}

	rand.Seed(time.Now().UnixNano())
	userAgent := userAgents[rand.Intn(len(userAgents))]

	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		msg := fmt.Sprintf("Ошибка создания запроса для %s: %v", link, err)
		connResults <- msg
		reportEntries <- HTMLReportEntry{URL: link, Success: false, Message: err.Error()}
		LogEvent(msg)
		return
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://www.google.com/")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("DNT", "1")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		msg := fmt.Sprintf("Не удалось подключиться к сайту %s: %v", link, err)
		connResults <- msg
		reportEntries <- HTMLReportEntry{URL: link, Success: false, Message: err.Error()}
		LogEvent(msg)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		msg := fmt.Sprintf("Успешное подключение к %s", link)
		connResults <- msg
		LogEvent(msg)
		cookieEntry := SaveCookiesToFile(link, resp.Cookies())
		jsonEntries <- cookieEntry
		cookieResults <- fmt.Sprintf("Куки для %s: %s", link, strings.Join(getCookieString(cookieEntry.Cookies), "; "))
		LogEvent(fmt.Sprintf("Куки для %s сохранены", link))
		reportEntries <- HTMLReportEntry{URL: link, Success: true, Message: "Успешное подключение"}
	} else {
		msg := fmt.Sprintf("Не удалось подключиться к сайту %s, статус: %d", link, resp.StatusCode)
		connResults <- msg
		reportEntries <- HTMLReportEntry{URL: link, Success: false, Message: fmt.Sprintf("HTTP статус: %d", resp.StatusCode)}
		LogEvent(msg)
	}
	time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)
}
