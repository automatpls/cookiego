package main

import (
	"bufio"
	"fmt"
	"net/http"
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
		link := scanner.Text()
		link = strings.TrimSpace(link) //

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

func CheckConnection(link string, wg *sync.WaitGroup) {
	defer wg.Done()

	client := http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get(link)
	if err != nil {
		fmt.Printf("Не удалось подключиться к сайту %s: %v\n", link, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("Успешное подключение к %s\n", link)
	} else {
		fmt.Printf("Не удалось подключиться к сайту %s, статус: %d\n", link, resp.StatusCode)
	}
}

// Объединение всех файлов вида links*.txt из папки загрузок
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

	var wg sync.WaitGroup

	fmt.Println("Список ссылок:")

	for _, link := range links {
		wg.Add(1)
		fmt.Println(link)
		go CheckConnection(link, &wg)
	}

	wg.Wait()
}
