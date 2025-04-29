package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
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

func ReadLinks(filename string) ([]string, error) {
	var links []string
	file, err := os.Open(filename)
	if err != nil {
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
		return nil, fmt.Errorf("ошибка чтения файла: %v", err)
	}
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
		connResults <- fmt.Sprintf("Ошибка создания запроса для %s: %v", link, err)
		reportEntries <- HTMLReportEntry{URL: link, Success: false, Message: err.Error()}
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
		connResults <- fmt.Sprintf("Не удалось подключиться к сайту %s: %v", link, err)
		reportEntries <- HTMLReportEntry{URL: link, Success: false, Message: err.Error()}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		connResults <- fmt.Sprintf("Успешное подключение к %s", link)
		cookieEntry := SaveCookiesToFile(link, resp.Cookies())
		jsonEntries <- cookieEntry
		cookieResults <- fmt.Sprintf("Куки для %s: %s", link, strings.Join(getCookieString(cookieEntry.Cookies), "; "))
		reportEntries <- HTMLReportEntry{URL: link, Success: true, Message: "Успешное подключение"}
	} else {
		connResults <- fmt.Sprintf("Не удалось подключиться к сайту %s, статус: %d", link, resp.StatusCode)
		reportEntries <- HTMLReportEntry{URL: link, Success: false, Message: fmt.Sprintf("HTTP статус: %d", resp.StatusCode)}
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

func WriteCookiesJSON(entries []CookieEntry) error {
	file, err := os.Create("OUTPUT/cookies.json")
	if err != nil {
		return fmt.Errorf("не удалось создать cookies.json: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
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
		return err
	}

	f, err := os.Create("OUTPUT/report.html")
	if err != nil {
		return err
	}
	defer f.Close()
	return t.Execute(f, entries)
}

func MergeDownloadedLinks() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("не удалось найти домашнюю папку: %v", err)
	}
	downloadsDir := homeDir + "/Downloads"
	files, err := os.ReadDir(downloadsDir)
	if err != nil {
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
				fmt.Printf("не удалось открыть файл %s: %v\n", name, err)
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
		return fmt.Errorf("не найдено подходящих файлов links*.txt в папке загрузок")
	}
	outputFile := "combined_links.txt"
	out, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("не удалось создать файл %s: %v", outputFile, err)
	}
	defer out.Close()
	for _, link := range allLinks {
		_, _ = out.WriteString(link + "\n")
	}
	fmt.Printf("Объединено %d ссылок в файл %s\n", len(allLinks), outputFile)
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
			return nil, fmt.Errorf("неправильный формат прокси: %v", err)
		}
		if proxyType == "2" {
			var auth *proxy.Auth
			if user != "" && pass != "" {
				auth = &proxy.Auth{User: user, Password: pass}
			}
			dialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
			if err != nil {
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
	err := MergeDownloadedLinks()
	if err != nil {
		fmt.Println("Ошибка при объединении ссылок:", err)
		return
	}
	links, err := ReadLinks("combined_links.txt")
	if err != nil {
		fmt.Println("Ошибка при чтении ссылок:", err)
		return
	}
	client, err := CreateHTTPClient()
	if err != nil {
		fmt.Println("Ошибка создания клиента:", err)
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
	} else {
		fmt.Println("Куки сохранены в cookies.json")
	}

	var reportData []HTMLReportEntry
	for r := range reportEntries {
		reportData = append(reportData, r)
	}
	if err := WriteHTMLReport(reportData); err != nil {
		fmt.Println("Ошибка создания HTML отчета:", err)
	} else {
		fmt.Println("HTML-отчет сохранен в report.html")
	}
}
