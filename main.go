package main

import (
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	"os"
)

type Event struct {
	Name  string
	Value int
	At    string
}

const batchThreshold = 100

var (
	db              *sql.DB
	eventBuffer     []Event
	eventBufferLock sync.Mutex
)

func init() {
	dataSourceName := os.Getenv("HAKARU_DATASOURCENAME")
	if dataSourceName == "" {
		dataSourceName = "root:password@tcp(127.0.0.1:13306)/hakaru"
	}

	var err error

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

/**
 * eventBuffer が閾値を超えたら Bulk Insert する
 * @query INSERT INTO eventlog(at, name, value) VALUES ('', '', ''), ('', '', ''), ('', '', '')
 */
func bulkInsert() {
	eventBufferLock.Lock()
	defer eventBufferLock.Unlock()

	if (len(eventBuffer)) == 0 {
		return
	}

	log.Printf("[BULK] Start: %d", len(eventBuffer))

	query := "INSERT INTO eventlog(at, name, value) VALUES"
	values := []interface{}{}

	for _, event := range eventBuffer {
		query += "(?, ?, ?),"
		values = append(values, event.At, event.Name, event.Value)
	}
	query = query[:len(query)-1] // 最後のカンマを削除

	// 直接 SQL を実行しているが動的プリペアドステートメントのため SQL インジェクションの心配はない
	_, err := db.Exec(query, values...)
	if err != nil {
		panic(err.Error())
	}

	eventBuffer = eventBuffer[:0] // バッファーをクリア

	log.Printf("[BULK] End: %d", len(eventBuffer))
}

func hakaruHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	value := r.URL.Query().Get("value")

	if value == "" {
		value = "1"
	}

	valueInt, err := strconv.Atoi(value)
	if err != nil {
		return
	}

	at := time.Now().Format("2006-01-02 15:04:05")

	eventBufferLock.Lock()
	eventBuffer = append(eventBuffer, Event{Name: name, Value: valueInt, At: at})
	if (len(eventBuffer)) >= batchThreshold {
		log.Printf("[HAKARU] Bulk Triggered: %d", len(eventBuffer))
		go bulkInsert()
	}
	eventBufferLock.Unlock()

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
