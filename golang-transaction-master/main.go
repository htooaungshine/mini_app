package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
)

var secretKey = []byte("your_secret_key")

func connectDB() (*sql.DB, error) {
	connStr := "user=postgres dbname=wallet_engine password=admin host=localhost port=5433 sslmode=disable"
	return sql.Open("postgres", connStr)
}

func authenticateToken(tokenString string) (bool, string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return secretKey, nil
	})
	if err != nil || !token.Valid {
		return false, "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return false, "", fmt.Errorf("invalid token claims")
	}
	return true, claims["username"].(string), nil
}

type Transaction struct {
	Sender         string  `json:"sender"`
	Receiver       string  `json:"receiver"`
	Amount         float64 `json:"amount"`
	IdempotencyKey string  `json:"idempotency_key"`
}

func transactionHandler(w http.ResponseWriter, r *http.Request) {
	db, err := connectDB()
	if err != nil {
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "No token provided", http.StatusForbidden)
		return
	}

	valid, username, err := authenticateToken(token)
	if !valid {
		http.Error(w, fmt.Sprintf("Invalid token: %v", err), http.StatusUnauthorized)
		return
	}

	var txn Transaction
	err = json.NewDecoder(r.Body).Decode(&txn)
	if err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	if txn.Sender != username {
		http.Error(w, "Sender does not match authenticated user", http.StatusForbidden)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	var senderID, receiverID int
	var senderBalance float64

	// Check for duplicate transaction using idempotency key
	var existingTransactionID int
	err = tx.QueryRow("SELECT id FROM transactions WHERE idempotency_key = $1", txn.IdempotencyKey).Scan(&existingTransactionID)
	if err != sql.ErrNoRows {
		if err == nil {
			http.Error(w, "Duplicate transaction", http.StatusConflict)
			return
		}
		http.Error(w, "Error checking idempotency", http.StatusInternalServerError)
		return
	}

	// Retrieve sender information
	fmt.Printf("Attempting to retrieve sender info for username: %s\n", txn.Sender)
	err = tx.QueryRow("SELECT pockets.id, pockets.balance FROM pockets JOIN users ON users.id = pockets.user_id WHERE users.username = $1 FOR UPDATE", txn.Sender).Scan(&senderID, &senderBalance)
	if err != nil {
		fmt.Printf("Error retrieving sender info: %v\n", err)
		if err == sql.ErrNoRows {
			http.Error(w, "Sender not found in database", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Error retrieving sender information: %v", err), http.StatusInternalServerError)
		}
		return
	}
	fmt.Printf("Successfully retrieved sender info - ID: %d, Balance: %f\n", senderID, senderBalance)

	if senderBalance < txn.Amount {
		http.Error(w, "Insufficient balance", http.StatusBadRequest)
		return
	}

	// Retrieve receiver information
	fmt.Printf("Attempting to retrieve receiver info for username: %s\n", txn.Receiver)
	err = tx.QueryRow("SELECT pockets.id FROM pockets JOIN users ON users.id = pockets.user_id WHERE users.username = $1 FOR UPDATE", txn.Receiver).Scan(&receiverID)
	if err != nil {
		fmt.Printf("Error retrieving receiver info: %v\n", err)
		if err == sql.ErrNoRows {
			http.Error(w, fmt.Sprintf("Receiver '%s' not found in database", txn.Receiver), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Error retrieving receiver information: %v", err), http.StatusInternalServerError)
		}
		return
	}
	fmt.Printf("Successfully retrieved receiver info - ID: %d\n", receiverID)

	// Update sender's balance
	_, err = tx.Exec("UPDATE pockets SET balance = balance - $1 WHERE user_id = $2", txn.Amount, senderID)
	if err != nil {
		return
	}

	// Update receiver's balance
	_, err = tx.Exec("UPDATE pockets SET balance = balance + $1 WHERE user_id = $2", txn.Amount, receiverID)
	if err != nil {
		return
	}

	// Insert transaction record
	_, err = tx.Exec("INSERT INTO transactions (sender_id, receiver_id, amount, idempotency_key) VALUES ($1, $2, $3, $4)", senderID, receiverID, txn.Amount, txn.IdempotencyKey)
	if err != nil {
		return
	}

	err = tx.Commit()
	if err != nil {
		http.Error(w, "Transaction commit failed", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("$%.2f transferred from %s to %s", txn.Amount, txn.Sender, txn.Receiver),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	http.HandleFunc("/process", transactionHandler)
	port := ":8080"
	fmt.Println("Golang service running on http://localhost" + port)
	http.ListenAndServe(port, nil)
}
