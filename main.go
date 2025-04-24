package main

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

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

func CheckConnection(link string, client *http.Client, wg *sync.WaitGroup, results chan<- string) {
	defer wg.Done()

	resp, err := client.Get(link)
	if err != nil {
		results <- fmt.Sprintf("Не удалось подключиться к сайту %s: %v\n", link, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		results <- fmt.Sprintf("Успешное подключение к %s\n", link)
		SaveCookiesToFile(link, resp.Cookies())
	} else {
		results <- fmt.Sprintf("Не удалось подключиться к сайту %s, статус: %d\n", link, resp.StatusCode)
	}
}

func SaveCookiesToFile(link string, cookies []*http.Cookie) {
	var cookieStr []string
	for _, cookie := range cookies {
		cookieStr = append(cookieStr, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}

	cookiesFile, err := os.OpenFile("cookies.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Ошибка записи куков в файл: %v\n", err)
		return
	}
	defer cookiesFile.Close()

	_, err = cookiesFile.WriteString(fmt.Sprintf("%s %s\n", link, strings.Join(cookieStr, "; ")))
	if err != nil {
		fmt.Printf("Ошибка записи куков в файл: %v\n", err)
	}

	fmt.Printf("Куки для %s: %s\n", link, strings.Join(cookieStr, "; "))
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

func CreateHTTPClient() (*http.Client, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Использовать прокси? (y/n): ")
	useProxyInput, _ := reader.ReadString('\n')
	useProxy := strings.TrimSpace(strings.ToLower(useProxyInput))

	if useProxy == "y" {
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

		transport := &http.Transport{
			Proxy: http.ProxyURL(parsedURL),
		}

		return &http.Client{
			Transport: transport,
			Timeout:   5 * time.Second,
		}, nil
	}

	return &http.Client{
		Timeout: 5 * time.Second,
	}, nil
}

func main() {
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

	fmt.Println("Список ссылок:")
	for _, link := range links {
		fmt.Println(link)
	}

	results := make(chan string, len(links))

	for _, link := range links {
		wg.Add(1)
		go CheckConnection(link, client, &wg, results)
	}

	wg.Wait()
	close(results)

	fmt.Println("\nРезультаты подключения и куков:")
	for result := range results {
		fmt.Print(result)
	}

	fmt.Println("Процесс завершен.")
}
