package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/julienschmidt/httprouter"
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
	URLs []string
}

func check(links links, query string) {
	for _, value := range links.URLs {
		data, err := http.Get(value)
		if err != nil {
		    log.Println(err)
		}

		var pageData []byte
		_, err = data.Body.Read(pageData)

		if strings.Contains(query, string(pageData)) {

		}
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
		Addr:    net.JoinHostPort("", port),
		Handler: router,
		//TLSConfig:         nil,
		ReadTimeout:       1 * time.Minute,
		ReadHeaderTimeout: 15 * time.Second,
		WriteTimeout:      1 * time.Minute,
		//IdleTimeout:       0,
		//MaxHeaderBytes:    0,
		//TLSNextProto:      nil,
		//ConnState:         nil,
		//ErrorLog:          nil,
		//BaseContext:       nil,
		//ConnContext:       nil,
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
func Welcome(w http.ResponseWriter, r *http.Request, actions httprouter.Params) {

	fmt.Fprint(w, "Hello, World!")
}
