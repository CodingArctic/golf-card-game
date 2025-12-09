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
	hostAddress := os.Getenv("HOST_ADDRESS")

	// create database connection pool
	db, err := database.NewPool(ctx, connectionString)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// create data access layer
	userRepo := database.NewUserRepository(db)
	chatRepo := database.NewChatRepository(db)
	gameRepo := database.NewGameRepository(db)

	// create business layer
	userService := business.NewUserService(userRepo)
	gameService := business.NewGameService(gameRepo, userRepo)
	nonceManager := business.NewNonceManager()
	emailService := service.NewEmailService()

	// Set the services for HTTP handlers
	service.SetUserService(userService)
	service.SetNonceManager(nonceManager)
	service.SetEmailService(emailService)
	service.SetChatRepository(chatRepo)
	service.SetGameRepository(gameRepo)
	service.SetGameService(gameService)

	// Start the chat hub as a background goroutine
	go service.Hub.Run()

	// a mux (multiplexer) routes incoming requests to their respective handlers
	mux := http.NewServeMux()

	// Public endpoints for authentication
	mux.HandleFunc("/api/register/nonce", service.GetRegistrationNonceHandler)
	mux.HandleFunc("/api/register", service.RegisterHandler)
	mux.HandleFunc("/api/login", service.LoginHandler)
	mux.HandleFunc("/api/logout", service.LogoutHandler)

	// Protected API endpoints

	// Game management
	mux.HandleFunc("/api/game/create", service.CreateGameHandler)
	mux.HandleFunc("/api/game/invite", service.InvitePlayerHandler)
	mux.HandleFunc("/api/game/accept", service.AcceptInvitationHandler)
	mux.HandleFunc("/api/game/decline", service.DeclineInvitationHandler)
	mux.HandleFunc("/api/game/list", service.ListGamesHandler)
	mux.HandleFunc("/api/game/details", service.GetGameHandler)

	// WebSocket endpoints
	mux.HandleFunc("/api/ws/chat", service.ChatHandler)
	mux.HandleFunc("/api/ws/game/", service.GameWebSocketHandler)

	// Serve static files from frontend/out directory with custom 404 handling
	mux.Handle("/", service.NotFoundHandler(http.Dir("./frontend/out")))

	// Wrap with session middleware
	protected := service.SessionMiddleware(mux)

	// If we hadn't created a custom mux to enable middleware,
	// the second param would be nil, which uses http.DefaultServeMux.
	log.Print("listening on: http://" + hostAddress)
	log.Fatal(http.ListenAndServe(hostAddress, protected))
}
