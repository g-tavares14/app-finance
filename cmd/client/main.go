package main

import (
	"app-finance/internal/models"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

func main() {

	req := models.CreateUser{
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john.doe@example.com",
		Password:  "password123",
	}

	body, err := json.Marshal(req)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := http.Post("http://localhost:8080/users", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		message, _ := io.ReadAll(resp.Body)
		log.Fatalf("failed to create user: %s: %s", resp.Status, bytes.TrimSpace(message))
	}

	var responseApi models.CreateUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&responseApi); err != nil {
		log.Fatal(err)
	}

	fmt.Println("User created", responseApi)
}
