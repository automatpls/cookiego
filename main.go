package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

type CookieEntry struct {
	URL     string            `json:"url"`
	Cookies map[string]string `json:"cookies"`
}

type HTMLReportEntry struct {
	URL     string
	Success bool
	Message string
}

type CustomTransport struct {
	Base http.RoundTripper
}

var logger *log.Logger

func LogEvent(message string) {
	if logger != nil {
		logger.Println(message)
	}
}

func ReadLinks(filename string) ([]string, error) {
	var links []string
	file, err := os.Open(filename)
	if err != nil {
		LogEvent(fmt.Sprintf("Ошибка открытия файла: %v", err))
		return nil, fmt.Errorf("ошибка открытия файла: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		link := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(link, "https://") {
			if !strings.Contains(link, ":") {
				link += ":443"
			}
		} else if strings.HasPrefix(link, "http://") {
			if !strings.Contains(link, ":") {
				link += ":80"
			}
		} else {
			link = "http://" + link + ":80"
		}
		links = append(links, link)
	}

	if err := scanner.Err(); err != nil {
		LogEvent(fmt.Sprintf("Ошибка чтения файла: %v", err))
		return nil, fmt.Errorf("ошибка чтения файла: %v", err)
	}
	LogEvent(fmt.Sprintf("Загружено %d ссылок из файла %s", len(links), filename))
	return links, nil
}

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
		return savedLinks // если файла нет — возвращаем пустую map
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

func WriteHTMLReport(entries []HTMLReportEntry) error {
	const tpl = `<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <title>Отчёт проверки сайтов</title>
    <style>
        body { font-family: Arial, sans-serif; }
        .success { color: green; }
        .fail { color: red; }
    </style>
</head>
<body>
<h1>Результаты проверки сайтов</h1>
<ul>
    {{range .}}
    <li class="{{if .Success}}success{{else}}fail{{end}}">{{.URL}} — {{.Message}}</li>
    {{end}}
</ul>
</body>
</html>`
	t, err := template.New("report").Parse(tpl)
	if err != nil {
		LogEvent(fmt.Sprintf("Ошибка парсинга шаблона отчета: %v", err))
		return err
	}

	f, err := os.Create("OUTPUT/report.html")
	if err != nil {
		LogEvent(fmt.Sprintf("Ошибка создания report.html: %v", err))
		return err
	}
	defer f.Close()
	LogEvent("HTML-отчет сохранен в report.html")
	err = t.Execute(f, entries)
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs("OUTPUT/report.html")
	if err != nil {
		LogEvent(fmt.Sprintf("Ошибка получения пути для report.html: %v", err))
	} else {
		openReport(absPath)
	}

	return nil
}

func openReport(path string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}

	if err := cmd.Start(); err != nil {
		LogEvent(fmt.Sprintf("Не удалось открыть отчет в браузере: %v", err))
	} else {
		LogEvent("HTML-отчет открыт в браузере")
	}
}
func MergeDownloadedLinks() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		LogEvent(fmt.Sprintf("Не удалось найти домашнюю папку: %v", err))
		return fmt.Errorf("не удалось найти домашнюю папку: %v", err)
	}
	downloadsDir := homeDir + "/Downloads"
	files, err := os.ReadDir(downloadsDir)
	if err != nil {
		LogEvent(fmt.Sprintf("Не удалось прочитать папку загрузок: %v", err))
		return fmt.Errorf("не удалось прочитать папку загрузок: %v", err)
	}
	var allLinks []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if strings.Contains(strings.ToLower(name), "links") && strings.HasSuffix(name, ".txt") {
			fullPath := downloadsDir + "/" + name
			f, err := os.Open(fullPath)
			if err != nil {
				LogEvent(fmt.Sprintf("Не удалось открыть файл %s: %v", name, err))
				continue
			}
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					allLinks = append(allLinks, line)
				}
			}
			f.Close()
		}
	}
	if len(allLinks) == 0 {
		msg := "Не найдено подходящих файлов links*.txt в папке загрузок"
		LogEvent(msg)
		return fmt.Errorf(msg)
	}
	outputFile := "combined_links.txt"
	out, err := os.Create(outputFile)
	if err != nil {
		LogEvent(fmt.Sprintf("Не удалось создать файл %s: %v", outputFile, err))
		return fmt.Errorf("не удалось создать файл %s: %v", outputFile, err)
	}
	defer out.Close()
	for _, link := range allLinks {
		_, _ = out.WriteString(link + "\n")
	}
	LogEvent(fmt.Sprintf("Объединено %d ссылок в %s", len(allLinks), outputFile))
	return nil
}

func (t *CustomTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Connection", "keep-alive")
	return t.Base.RoundTrip(req)
}

func CreateHTTPClient() (*http.Client, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Использовать прокси? (y/n): ")
	useProxyInput, _ := reader.ReadString('\n')
	useProxy := strings.TrimSpace(strings.ToLower(useProxyInput))

	transport := &http.Transport{}
	if useProxy == "y" {
		fmt.Print("Введите тип прокси (1 - HTTP, 2 - SOCKS5): ")
		proxyTypeInput, _ := reader.ReadString('\n')
		proxyType := strings.TrimSpace(proxyTypeInput)
		fmt.Print("Введите адрес прокси (ip:порт): ")
		addr, _ := reader.ReadString('\n')
		addr = strings.TrimSpace(addr)
		fmt.Print("Введите логин (если есть, иначе Enter): ")
		user, _ := reader.ReadString('\n')
		user = strings.TrimSpace(user)
		fmt.Print("Введите пароль (если есть, иначе Enter): ")
		pass, _ := reader.ReadString('\n')
		pass = strings.TrimSpace(pass)
		var proxyURL string
		if user != "" && pass != "" {
			proxyURL = fmt.Sprintf("http://%s:%s@%s", user, pass, addr)
		} else {
			proxyURL = fmt.Sprintf("http://%s", addr)
		}
		parsedURL, err := url.Parse(proxyURL)
		if err != nil {
			LogEvent(fmt.Sprintf("Неправильный формат прокси: %v", err))
			return nil, fmt.Errorf("неправильный формат прокси: %v", err)
		}
		if proxyType == "2" {
			var auth *proxy.Auth
			if user != "" && pass != "" {
				auth = &proxy.Auth{User: user, Password: pass}
			}
			dialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
			if err != nil {
				LogEvent(fmt.Sprintf("Не удалось подключиться к SOCKS5: %v", err))
				return nil, fmt.Errorf("не удалось подключиться к SOCKS5: %v", err)
			}
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		} else {
			transport.Proxy = http.ProxyURL(parsedURL)
		}
	}
	return &http.Client{
		Transport: &CustomTransport{Base: transport},
		Timeout:   time.Second * 10,
	}, nil
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
