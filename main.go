package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/handlers"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var db *sql.DB

type Book struct {
	ID        uuid.UUID `json:"id"`
	ISBN      string    `json:"isbn"`
	Name      string    `json:"name"`
	Author    string    `json:"author"`
	Year      string    `json:"year"`
	Publisher string    `json:"publisher"`
}

func initDB() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file")
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=require",
		os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"), os.Getenv("DB_NAME"))

	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("connected to database")
}

func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func check(w http.ResponseWriter, _ *http.Request) {
	response := map[string]interface{}{
		"message": "OK",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Println("service is not responding", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func createBook(w http.ResponseWriter, r *http.Request) {
	var book Book
	err := json.NewDecoder(r.Body).Decode(&book)
	if err != nil {
		log.Println("error decoding book json", err)
		http.Error(w, "invalid request payload", http.StatusBadRequest)
	}

	query := `INSERT INTO books (isbn, name, author, year, publisher) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	err = db.QueryRow(query, book.ISBN, book.Name, book.Author, book.Year, book.Publisher).Scan(&book.ID)
	if err != nil {
		log.Println("error creating book:", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(book); err != nil {
		log.Println("error encoding response:", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func getBooks(w http.ResponseWriter, _ *http.Request) {
	rows, err := db.Query("SELECT id, isbn, name, author, YEAR, publisher FROM books")
	if err != nil {
		log.Println("error getting books", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Println("error closing rows", err)
		}
	}()

	var books []Book
	for rows.Next() {
		var book Book
		if err := rows.Scan(&book.ID, &book.ISBN, &book.Name, &book.Author, &book.Year, &book.Publisher); err != nil {
			log.Println("error getting books", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		books = append(books, book)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(books); err != nil {
		log.Println("error enconding books")
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func main() {
	initDB()

	r := mux.NewRouter()
	r.Use(logger)

	corsOrigins := handlers.AllowedOrigins([]string{"*"})
	corsMethods := handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	corsHeaders := handlers.AllowedHeaders([]string{"Content-Type", "Authorization"})

	r.HandleFunc("/api", check).Methods("GET")
	r.HandleFunc("/api/books", createBook).Methods("POST")
	r.HandleFunc("/api/books", getBooks).Methods("GET")

	handler := handlers.CORS(corsOrigins, corsMethods, corsHeaders)(r)

	port := "8080"
	fmt.Println("server running on port", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
