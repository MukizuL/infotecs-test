package main

import (
	"database/sql"
	"errors"
	"github.com/MukizuL/infotecs-test/internal/storage"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
)

type application struct {
	data storage.StorageConn
}

func main() {
	addr := getEnv("ADDR", ":8080")
	dsn := getEnv("DSN", "postgres://localhost/postgres")
	log.Println(addr, dsn)

	db, err := openDb(dsn)
	if err != nil {
		log.Fatal(errors.New("ошибка открытия базы данных"))
	}

	defer db.Close()

	app := &application{
		data: storage.StorageConn{DB: db},
	}

	app.data.Init()

	log.Print("Запускаем сервер ", addr)

	err = http.ListenAndServe(addr, app.routes())
	if err != nil {
		log.Fatal(errors.New("ошибка старта сервера"))
	}
}

func openDb(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
