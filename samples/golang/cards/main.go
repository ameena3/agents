// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	app := NewAgentApp()
	RegisterHandlers(app)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3978"
	}

	log.Printf("Cards agent listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, app.Router()))
}
