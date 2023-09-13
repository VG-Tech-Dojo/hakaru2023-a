package main

import (
	"log"
	"net/http"

	"database/sql"

	"os"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func init() {
	var err error

	dataSourceName := os.Getenv("HAKARU_DATASOURCENAME")
	if dataSourceName == "" {
		dataSourceName = "root:password@tcp(127.0.0.1:13306)/hakaru"
	}

	db, err = sql.Open("mysql", dataSourceName) // Connection Pool を作成
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}

	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(30)
}

func main() {
	http.HandleFunc("/hakaru", hakaruHandler)
	http.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	// start server
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}

func hakaruHandler(w http.ResponseWriter, r *http.Request) {
	stmt, e := db.Prepare("INSERT INTO eventlog(at, name, value) values(NOW(), ?, ?)")
	if e != nil {
		panic(e.Error())
	}

	defer stmt.Close()

	name := r.URL.Query().Get("name")
	value := r.URL.Query().Get("value")

	stats := db.Stats()

	waitCount := stats.WaitCount
	waitDuration := stats.WaitDuration
	log.Printf("waitCount: %d, waitDuration: %v", waitCount, waitDuration)

	_, err := stmt.Exec(name, value)
	if err != nil {
		panic(err.Error())
	}

	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
}
