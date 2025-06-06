package main

import (
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type HTMLReportEntry struct {
	URL     string
	Success bool
	Message string
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
