package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/csotherden/strftime"
	"golang.org/x/net/html"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const VersionFileName = "version.json"

// Version Структура для хранения версии.
type Version struct {
	Version string `json:"version"`
}

// BookstackPageDetails - Структура для хранения деталей страницы.
type BookstackPageDetails struct {
	BookId  int    `json:"book_id"`
	Id      int    `json:"id"`
	Html    string `json:"html"`
	RawHtml string `json:"raw_html"`
}

// CalculatedVersion - Вычисленная версия.
var CalculatedVersion string

func main() {
	if len(os.Args) == 0 {
		log.Fatalf("Не переданы параметры")
	}

	version, bookId, id := os.Args[1], os.Args[2], os.Args[3]

	log.Printf(`Версия из входного параметра: %v`, version)

	// Файла нету, создаем.
	if isNotExist := checkFileExists(version); isNotExist {
		createVersionJsonFile(version)
	}

	bookIdInt, _ := strconv.Atoi(bookId)
	idInt, _ := strconv.Atoi(id)

	changeAndSaveHtmlVersionBookstack(bookIdInt, idInt)
}

/*
checkFileExists Проверяем существование файла.
*/
func checkFileExists(filename string) bool {
	_, err := os.Stat(filename)

	if os.IsNotExist(err) {
		log.Printf(`Файл существует. Он будет перезаписан.`)

		return true
	}

	log.Printf(`Файл не существует. Он будет создан.`)

	return false
}

/*
createVersionJsonFile Создает json-файл для хранения версии.
*/
func createVersionJsonFile(versionFromInput string) {
	CalculatedVersion = "1.1." + versionFromInput + "." + generateSha1FromCurrentDateAndTime()

	version := Version{
		Version: CalculatedVersion,
	}

	jsonData, err := json.MarshalIndent(version, "", "  ") // "" для префикса, " " для отступов.

	if err != nil {
		log.Fatalf("Ошибка парсинга JSON: %v", err)
	}

	err = os.WriteFile(VersionFileName, jsonData, 0644) // 0644 задаем разрешения файлу.

	if err != nil {
		log.Fatalf("Ошибка записи в файл JSON: %v", err)
	}

	log.Printf("Данные записали в файл %v ", VersionFileName)
}

/*
generateSha1FromCurrentDateAndTime Создает строку по SHA-1 от текущей даты и времени.
*/
func generateSha1FromCurrentDateAndTime() string {
	// Создаем новый SHA-1 hash объект.
	hasher := sha1.New()

	hasherDate := getCurrentLocalDateTime()

	// Записываем строку даты и времени в слайс байтов.
	hasher.Write([]byte(hasherDate))

	// Получаем хэш-сумму из слайса байтов.
	sha1HashBytes := hasher.Sum(nil)

	// Конвертируем слайсс байтов в hexadecimal-строку.
	sha1HexString := fmt.Sprintf("%x", sha1HashBytes)

	log.Printf("SHA-1 строка: %v", sha1HexString)

	return sha1HexString
}

/*
changeAndSaveHtmlVersionBookstack - Изменяет html данные в Bookstack.
*/
func changeAndSaveHtmlVersionBookstack(bookId, id int) {
	// Получаем Html страницы с букстека и будем дополнять его.
	htmlContent := getHtmlFromBookstack()

	// Изменяем html.
	changedHtml := changeHtml(htmlContent.Html)

	// Сохраняем изменения в букстек.
	updateVersionBookstack(changedHtml, bookId, id)
}

/*
changeHtml - Изменяет html страницы.
*/
func changeHtml(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))

	if err != nil {
		log.Fatalf("Ошибка парсинга html-контента %v\n", err)
	}

	var f func(*html.Node)

	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			dateTime := getCurrentLocalDateTime()
			addRow(n, []string{dateTime, CalculatedVersion})

			parseTable(n)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}

	f(doc)

	var buf bytes.Buffer
	err = html.Render(&buf, doc)

	if err != nil {
		log.Fatalf("Ошибка при Render %v\n", err)
	}

	result := buf.String()

	log.Println(result)

	return result
}

/*
updateVersionBookstack - ОБновляет страницу версий в Bookstack.
*/
func updateVersionBookstack(changedHtml string, bookId, id int) {
	changedVersion := BookstackPageDetails{
		BookId:  bookId,
		Id:      id,
		Html:    changedHtml,
		RawHtml: changedHtml,
	}

	jsonBytes, err := json.Marshal(changedVersion)

	if err != nil {
		log.Fatalf("Ошибка при подготовке запроса на обновление страницы с Bookstack: %v\n", err)
	}

	req, err := http.NewRequest("PUT", "https://bookstack-ext.ntcees.ru/api/pages/139", bytes.NewBuffer(jsonBytes))

	if err != nil {
		log.Fatalf("Ошибка подготовки запроса на обновление страницы Bookstack: %v\n", err)
	}

	setHttpHeaders(req)

	// Создаем HTTP client.
	client := &http.Client{}

	// Выполняем запрос.
	resp, err := client.Do(req)

	if err != nil {
		log.Fatalf("Ошибка отправки запроса на обновление страницы Bookstack: %v\n", err)
	}

	// Дефер закроет подключение при выходе из функции.
	defer resp.Body.Close()

	log.Print("Успешно изменили страницу версий")
}

/*
parseTable - Парсит таблицу.
*/
func parseTable(tableNode *html.Node) {
	var rows [][]string

	for c := tableNode.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "tbody" || c.Data == "thead") {
			for r := c.FirstChild; r != nil; r = r.NextSibling {
				if r.Type == html.ElementNode && r.Data == "tr" {
					var rowData []string

					for cell := r.FirstChild; cell != nil; cell = cell.NextSibling {
						if cell.Type == html.ElementNode && (cell.Data == "td" || cell.Data == "th") {
							// Получение текста из ячейки.
							if cell.FirstChild != nil && cell.FirstChild.Type == html.TextNode {
								rowData = append(rowData, cell.FirstChild.Data)
							} else {
								// Обработка пустых ячеек.
								rowData = append(rowData, "")
							}
						}
					}

					rows = append(rows, rowData)
				}
			}
		}
	}
}

/*
addRow - Добавляет новую строку в таблицу.
*/
func addRow(tableNode *html.Node, data []string) {
	newRow := &html.Node{Type: html.ElementNode, Data: "tr"}

	for _, cellData := range data {
		newCell := &html.Node{Type: html.ElementNode, Data: "td"}
		newCell.AppendChild(&html.Node{Type: html.TextNode, Data: cellData})
		newRow.AppendChild(newCell)
	}

	// Если таблица имеет tbody, добавляем в tbody.
	//В противном случае добавляем прямо в таблицу.
	if tbody := findElement(tableNode, "tbody"); tbody != nil {
		tbody.AppendChild(newRow)
	} else {
		tableNode.AppendChild(newRow)
	}
}

/*
findElement - Находит элемент таблицы.
*/
func findElement(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElement(c, tag); found != nil {
			return found
		}
	}

	return nil
}

/*
getHtmlFromBookstack - Получает контент страницы с Bookstack.
*/
func getHtmlFromBookstack() (pageDetails BookstackPageDetails) {
	req, err := http.NewRequest("GET", "https://bookstack-ext.ntcees.ru/api/pages/139", nil)

	if err != nil {
		log.Fatalf("Ошибка подготовки запроса на получение страницы с Bookstack: %v\n", err)
	}

	setHttpHeaders(req)

	// Создаем HTTP client.
	client := &http.Client{}

	// Выполняем запрос.
	resp, err := client.Do(req)

	if err != nil {
		log.Fatalf("Ошибка отправки запроса на получение страницы с Bookstack: %v\n", err)
	}

	// Дефер закроет подключение при выходе из функции.
	defer resp.Body.Close()

	// Читаем результаты запроса.
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		log.Fatalf("Ошибка получения результатов запроса на получение страницы с Bookstack: %v\n", err)
	}

	var result BookstackPageDetails

	// Парсим результат на структуру.
	err = json.Unmarshal(body, &result)

	if err != nil {
		log.Fatalf("Ошибка парсинга к json %v\n:", err)
	}

	log.Printf("Результаты запроса на получение страницы с Bookstack: %s\n", body)

	return result
}

/*
setHttpHeaders - Записывает заголовки HTTP-запроса.
*/
func setHttpHeaders(req *http.Request) {
	req.Header.Add("Authorization", "Token L0oSjMWvNDxqjhYPhdCSKsvgDsBifqCj:6iTEfYJKZ2RlIG99NOBLvCnFLGzPVPsR")
	req.Header.Add("Content-Type", "application/json")
}

/*
getCurrentLocalDateTime - Выдает локальную дату и время с форматированием.
*/
func getCurrentLocalDateTime() string {
	return strftime.Format("%d.%m.%Y", time.Now().Local())
}
