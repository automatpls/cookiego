package main

import (
	"bufio"
	"fmt"
	"net"
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
		if !strings.Contains(link, ":") {
			link += ":80"
		}
		links = append(links, link)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("ошибка чтения файла: %v", err)
	}

	return links, nil
}

// Проверяем подключение к ссылке
func CheckConnection(link string, wg *sync.WaitGroup) {
	defer wg.Done()

	conn, err := net.DialTimeout("tcp", link, 3*time.Second)
	if err != nil {
		fmt.Printf("Не удалось подключиться к сайту %s: %v\n", link, err)
		return
	}
	conn.Close()
	fmt.Printf("Успешное подключение к %s\n", link)
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
