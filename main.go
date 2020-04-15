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
	"syscall"
	"time"
)

const (
	port = "8080"
)

var (
	moscow *time.Location
)

type links struct {
	URLs map[string]string
}

func check(links *links, query string) {

	for key, value := range (*links).URLs {
		fmt.Println("====== Проверяем URL", value)
		req, err := http.Get(value)
		if err != nil {
			log.Println(err)
		}

		var pageData []byte
		pageData, err = ioutil.ReadAll(req.Body)

		//fmt.Println("Тело:", string(pageData))

		if !strings.Contains(string(pageData), query) {
			fmt.Println("Поисковая строка не обнаружена")
			delete((*links).URLs, key)
			continue
		}

		fmt.Println("Поисковая строка обнаружена! Хорошо!")
	}
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

	// объявляем роутер
	var router *Middleware
	router = newMiddleware(
		httprouter.New(),
	)

	// анонсируем хандлеры
	SetUpHandlers(router)

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

func SetUpHandlers(m *Middleware) {
	fmt.Println("Setting up handlers...")

	// главная страница
	m.router.GET("/", Welcome)
	m.router.POST("/", Welcome)
}

// мидлвейр для всех хэндлеров
func (rw *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("-------------------", time.Now().In(moscow).Format(http.TimeFormat), "A request is received -------------------")
	log.Println("The request is from", r.RemoteAddr, "| Method:", r.Method, "| URI:", r.URL.String())

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

	rw.router.ServeHTTP(w, r)
}

// Welcome is the homepage of the service
func Welcome(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var err error
	switch r.Method {
	case "POST":
		var whatToSearch string

		whatToSearch = r.PostForm.Get("string")
		fmt.Println("Поисковая строка:", whatToSearch)

		if whatToSearch == "" {
			fmt.Println("Empty request")
			http.Error(w, http.StatusText(400), 400)
			return
		}

		_, err = fmt.Fprintln(w, "Ниже список ссылак на страницы, в которых найдена поисковая строка:", whatToSearch)
		if err != nil {
			http.Error(w, http.StatusText(503), 503)
			log.Println(err)
		}

		var links links

		links.URLs = make(map[string]string, 10)

		links.URLs = map[string]string{
			"3lines":  "https://3lines.club/",
			"Aramake": "https://www.aramake.ru/",
		}

		check(&links, whatToSearch)

		for _, value := range links.URLs {
			_, err = fmt.Fprintln(w, value)
			if err != nil {
				http.Error(w, http.StatusText(503), 503)
				log.Println(err)
			}
		}
	case "GET":
		_, err = fmt.Fprint(w,
			`

<html lang="ru">
<head>
    <meta charset="UTF-8">
</head>
<body>
	<form action="/" method="post">
		<label for="string">Поисковая строка:</label>
		<input type="text" id="string" name="string" placeholder="Лалала">
		<input type="submit" value="Искать">
	</form>
</body>
</html>

`)
		if err != nil {
			http.Error(w, http.StatusText(503), 503)
			log.Println(err)
		}
	}
}
