package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	port = "8080"
)

var (
	moscow *time.Location
)

// объекты и методы для хранения паролей
type Links struct {
	mx   sync.RWMutex
	URLs map[string]string
	Lock bool
}

// Lock set
func (c *Links) SetLock() {
	c.mx.Lock()
	defer c.mx.Unlock()

	// замок
	c.Lock = true
}

// CheckLock returns Lock status
func (c *Links) Unlocked() bool {
	c.mx.RLock()
	defer c.mx.RUnlock()

	// замок
	return !c.Lock
}

// Delete an URL form the map
func (c *Links) SetUnlock() {
	c.mx.Lock()
	defer c.mx.Unlock()

	// снимаем замок
	c.Lock = false

}

func NewAnswers() *Links {
	return &Links{
		URLs: make(map[string]string),
		Lock: false,
	}
}

// Delete an URL form the map
func (c *Links) DeleteUser(URLkey string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	// удаляем по ключу
	delete(c.URLs, URLkey)
}

// AddURL adds an URL to the map
func (c *Links) AddURL(key, value string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	// добавляем URL
	c.URLs[key] = value
}

// Print prints the URLs
func (c *Links) Print(w http.ResponseWriter) error {
	c.mx.RLock()
	defer c.mx.RUnlock()

	// добавляем URL
	for _, value := range c.URLs {
		_, err := fmt.Fprintln(w, value)
		if err != nil {
			http.Error(w, http.StatusText(503), 503)
			log.Println(err)
			return err
		}
	}

	return nil
}

// GetMap returns the URLs
func (c *Links) GetMap() map[string]string {
	c.mx.RLock()
	defer c.mx.RUnlock()

	return c.URLs
}

// EraseData removes all the data from the map
func (c *Links) EraseUserData() {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.URLs = make(map[string]string)
}

func ProcessURL(key, URL, query string, resultURLs *Links) {
	fmt.Println("====== Проверяем URL", URL)
	req, err := http.Get(URL)
	if err != nil {
		log.Println(err)
		return
	}

	var pageData []byte
	pageData, err = ioutil.ReadAll(req.Body)

	if !strings.Contains(string(pageData), query) {
		fmt.Println("Поисковая строка не обнаружена")
		return
	}

	fmt.Println("Поисковая строка обнаружена! Хорошо!")
	resultURLs.AddURL(key, URL)
}

func check(sourceURLs, resultURLs *Links, query string) error {

	// указываем что обработка данных не завершена (на случай олимпиарда ссылок)
	resultURLs.SetLock()

	// безопасно вычитываем карту
	tempMap := sourceURLs.GetMap()

	// проходимся по ней
	for key, value := range tempMap {
		go ProcessURL(key, value, query, resultURLs)
	}

	// указываем что обработка завершена
	resultURLs.SetUnlock()
	return nil
}

// Middleware wraps julien's router http methods
type Middleware struct {
	router *httprouter.Router
	db     *sql.DB
}

// newMiddleware returns pointer of Middleware
func newMiddleware(r *httprouter.Router) *Middleware {
	var db *sql.DB
	return &Middleware{r, db}
}

func main() {

	var err error

	// Устанавливаем сдвиг времени
	moscow, _ = time.LoadLocation("Europe/Moscow")

	// Переменная для исходных URL
	var sourceURLs = NewAnswers()

	// Переменная для накопления результата по поиску по URL
	var resultURL = NewAnswers()

	// устанавливаем исходные URL
	sourceURLs.URLs = map[string]string{
		"3lines":  "https://3lines.club/",
		"Aramake": "https://www.aramake.ru/",
	}

	// объявляем роутер
	var router *Middleware
	router = newMiddleware(
		httprouter.New(),
	)

	// анонсируем хандлеры
	router.router.GET("/", Welcome(sourceURLs, resultURL))
	router.router.GET("/:action", Welcome(sourceURLs, resultURL))
	router.router.POST("/", Welcome(sourceURLs, resultURL))

	webServer := http.Server{
		Addr:              net.JoinHostPort("", port),
		Handler:           router,
		ReadTimeout:       1 * time.Minute,
		ReadHeaderTimeout: 15 * time.Second,
		WriteTimeout:      1 * time.Minute,
	}

	fmt.Println("Launching the service on the port:", port, "...")
	go func() {
		err = webServer.ListenAndServe()
		if err != nil {
			log.Println(err)
		}
	}()

	fmt.Println("The server was launched!")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	<-interrupt

	timeout, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()

	err = webServer.Shutdown(timeout)
	if err != nil {
		log.Println(err)
	}

}

// мидлвейр для всех хэндлеров
func (rw *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("-------------------", time.Now().In(moscow).Format(http.TimeFormat), "A request is received -------------------")
	fmt.Println("The request is from", r.RemoteAddr, "| Method:", r.Method, "| URI:", r.URL.String())

	if r.Method == "POST" {
		// проверяем размер POST данных
		r.Body = http.MaxBytesReader(w, r.Body, 1000)
		err := r.ParseForm()
		if err != nil {
			fmt.Println("POST data is exceeded the limit")
			http.Error(w, http.StatusText(400), 400)
			return
		}
	}
	fmt.Fprintln(w, "<html lang=ru><head><meta charset=UTF-8></head><body>")
	rw.router.ServeHTTP(w, r)
	fmt.Fprint(w, "</body></html>")

}

// Welcome is the homepage of the service
func Welcome(sourceURLs, resultURLs *Links) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, actions httprouter.Params) {
		var err error
		switch {
		case actions.ByName("action") == "check":

			fmt.Fprintln(w, "Результаты:<br>")

			err = resultURLs.Print(w)
			if err != nil {
				http.Error(w, http.StatusText(503), 503)
				return
			}

			fmt.Fprintln(w, "<br><br><a href=/>Новый запрос</a>")
		case r.Method == "POST":
			var whatToSearch string

			whatToSearch = r.PostForm.Get("string")
			fmt.Println("Поисковая строка:", whatToSearch)

			if whatToSearch == "" {
				fmt.Println("Empty request")
				http.Error(w, http.StatusText(400), 400)
				return
			}

			_, err = fmt.Fprintln(w, "Ищем фращу:", whatToSearch)
			if err != nil {
				http.Error(w, http.StatusText(503), 503)
				log.Println(err)
			}

			if resultURLs.Unlocked() {
				err = check(sourceURLs, resultURLs, whatToSearch)
				_, err = fmt.Fprintln(w, "<br>Обработка запущена...")
			} else {
				_, err = fmt.Fprintln(w, "<br><br>Обработка уже запущена и ещё не завершена...")
			}

			fmt.Fprintln(w, "<a href=/check>Просмотр результатов</a>")

		case r.Method == "GET":
			_, err = fmt.Fprint(w, `<form action="/" method="post">
						<label for="string">Поисковая строка:</label>
						<input type="text" id="string" name="string" placeholder="Лалала">
						<input type="submit" value="Искать">
					</form>`)
			fmt.Fprintln(w, "<a href=/check>Просмотр текущих результатов (если ранее запускали)</a>")
			if err != nil {
				http.Error(w, http.StatusText(503), 503)
				log.Println(err)
			}
		}
	}
}
