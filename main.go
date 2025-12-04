package main

import (
	"context"
	"golf-card-game/business"
	"golf-card-game/database"
	"golf-card-game/service"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	ctx := context.Background()

	// load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	connectionString := os.Getenv("CONNECTION_STRING")

	// create database connection pool
	db, err := database.NewPool(ctx, connectionString)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// create data access layer
	userRepo := database.NewUserRepository(db)
	chatRepo := database.NewChatRepository(db)

	// create business layer
	userService := business.NewUserService(userRepo)

	// Set the user service for HTTP handlers
	service.SetUserService(userService)
	service.SetChatRepository(chatRepo)

	// Start the chat hub as a background goroutine
	go service.Hub.Run()

	// a mux (multiplexer) routes incoming requests to their respective handlers
	mux := http.NewServeMux()

	// Public endpoints for authentication
	mux.HandleFunc("/api/register", service.RegisterHandler)
	mux.HandleFunc("/api/login", service.LoginHandler)
	mux.HandleFunc("/api/logout", service.LogoutHandler)

	// Protected API endpoints
	// mux.HandleFunc("/api/turn", service.GetTurnHandler)
	// mux.HandleFunc("/api/next", service.NextTurnHandler)
	mux.HandleFunc("/api/ws/chat", service.ChatHandler)

	// Serve Next.js static export from the 'out' directory
	fs := http.FileServer(http.Dir("./frontend/out"))
	mux.Handle("/", fs)

	// Wrap with session middleware
	protected := service.SessionMiddleware(mux)

	// If we hadn't created a custom mux to enable middleware,
	// the second param would be nil, which uses http.DefaultServeMux.
	log.Print("listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe("localhost:8080", protected))
}
