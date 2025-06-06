package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

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
