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

// Чтение файла и запись ссылок в массив
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
		link = strings.TrimSpace(link) // Убираем лишние пробелы

		// Проверяем, начинается ли ссылка с https://
		if strings.HasPrefix(link, "https://") {
			// Если есть https, добавляем порт 443
			if !strings.Contains(link, ":") {
				link += ":443"
			}
		} else if strings.HasPrefix(link, "http://") {
			// Если есть http, добавляем порт 80
			if !strings.Contains(link, ":") {
				link += ":80"
			}
		} else {
			// Если ссылка не содержит протокол, добавляем http:// и порт 80
			link = "http://" + link + ":80"
		}

		links = append(links, link)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("ошибка чтения файла: %v", err)
	}

	return links, nil
}

// Проверка подключения к ссылке с использованием пакета net/http
func CheckConnection(link string, wg *sync.WaitGroup) {
	defer wg.Done()

	// Устанавливаем таймаут для HTTP-запроса
	client := http.Client{
		Timeout: 3 * time.Second,
	}

	// Отправляем GET-запрос
	resp, err := client.Get(link)
	if err != nil {
		fmt.Printf("Не удалось подключиться к сайту %s: %v\n", link, err)
		return
	}
	defer resp.Body.Close()

	// Проверяем, если статус код ответа успешный
	if resp.StatusCode == http.StatusOK {
		fmt.Printf("Успешное подключение к %s\n", link)
	} else {
		fmt.Printf("Не удалось подключиться к сайту %s, статус: %d\n", link, resp.StatusCode)
	}
}

func main() {
	links, err := ReadLinks("links.txt")
	if err != nil {
		fmt.Println("Ошибка:", err)
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
