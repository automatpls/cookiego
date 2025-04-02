package main

import (
	"bufio"
	"fmt"
	"os"
)

// Чтение файла и запись ссылок в массив
func ReadLinks(filename string) ([]string, error) {
	//Массив для ссылок
	var links []string

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла: %v", err)
	}
	defer file.Close()
	//Построчная их запись через scanner
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		link := scanner.Text()
		links = append(links, link)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("ошибка чтения файла: %v", err)
	}

	return links, nil
}

func main() {
	links, err := ReadLinks("links.txt")
	if err != nil {
		fmt.Println("Ошибка:", err)
		return
	}

	fmt.Println("Список ссылок:")
	for _, link := range links {
		fmt.Println(link)
	}
}
