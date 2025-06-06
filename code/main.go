package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var outputDir string

func init() {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Не удалось получить путь к .exe: %v", err)
	}
	baseDir := filepath.Dir(exePath)
	outputDir = filepath.Join(baseDir, "OUTPUT")

	// Создаём папку OUTPUT, если её нет
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		os.MkdirAll(outputDir, 0755)
	}
}

var logger *log.Logger

func LogEvent(message string) {
	if logger != nil {
		logger.Println(message)
	}
}

func main() {
	os.MkdirAll("OUTPUT", 0755)

	logFile, err := os.OpenFile("OUTPUT/log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("Ошибка открытия log.txt:", err)
		return
	}
	defer logFile.Close()
	logger = log.New(io.MultiWriter(logFile), "", log.LstdFlags)

	LogEvent("Программа запущена")

	err = MergeDownloadedLinks()
	if err != nil {
		fmt.Println("Ошибка при объединении ссылок:", err)
		LogEvent("Остановка из-за ошибки объединения ссылок")
		return
	}

	links, err := ReadLinks("combined_links.txt")
	if err != nil {
		fmt.Println("Ошибка при чтении ссылок:", err)
		LogEvent("Остановка из-за ошибки чтения ссылок")
		return
	}

	client, err := CreateHTTPClient()
	if err != nil {
		fmt.Println("Ошибка создания клиента:", err)
		LogEvent("Остановка из-за ошибки создания клиента")
		return
	}

	var wg sync.WaitGroup
	connResults := make(chan string, len(links))
	cookieResults := make(chan string, len(links))
	jsonEntries := make(chan CookieEntry, len(links))
	reportEntries := make(chan HTMLReportEntry, len(links))

	fmt.Println("Список ссылок:")
	for _, link := range links {
		fmt.Println(link)
		wg.Add(1)
		go CheckConnection(link, client, &wg, connResults, cookieResults, jsonEntries, reportEntries)
	}

	go func() {
		wg.Wait()
		close(connResults)
		close(cookieResults)
		close(jsonEntries)
		close(reportEntries)
	}()

	fmt.Println("\nРезультаты подключения:")
	for res := range connResults {
		fmt.Println(res)
	}

	fmt.Println("\nКуки:")
	for cookie := range cookieResults {
		fmt.Println(cookie)
	}

	var allEntries []CookieEntry
	for entry := range jsonEntries {
		allEntries = append(allEntries, entry)
	}
	if err := WriteCookiesJSON(allEntries); err != nil {
		fmt.Println("Ошибка записи cookies.json:", err)
		LogEvent("Ошибка при записи cookies.json")
	} else {
		fmt.Println("Куки сохранены в cookies.json")
	}
	var reportData []HTMLReportEntry
	for r := range reportEntries {
		reportData = append(reportData, r)
	}
	if err := WriteHTMLReport(reportData); err != nil {
		fmt.Println("Ошибка создания HTML отчета:", err)
		LogEvent("Ошибка создания HTML отчета")
	} else {
		fmt.Println("HTML-отчет сохранен в report.html")
	}
	LogEvent("Done")
}
