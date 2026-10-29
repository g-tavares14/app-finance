package main

import (
	"app-finance/internal/models"
	"bytes"
	"encoding/json"
	"fmt"
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
		return
	}

	resp, err := http.Post("http://localhost:8080/users", "application/json", bytes.NewReader(body))
	if err != nil {
		panic(err)
		return
	}

	if resp.StatusCode != http.StatusCreated {
		panic("failed to create user")
	}

	var responseApi models.CreateUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&responseApi); err != nil {
		panic(err)
	}

	fmt.Println("User created", responseApi)
}
