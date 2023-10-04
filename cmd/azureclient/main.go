package main

import (
	"azuread-play/internal/azureclient"
	"context"
	"os"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

func main() {
	ctx := context.Background()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	// create a logger and add it to the context
	log := zerolog.New(os.Stdout).With().Caller().Timestamp().Logger()
	ctx = log.WithContext(ctx)

	err := godotenv.Overload()
	if err != nil {
		log.Fatal().Err(err).Msg("Error loading .env file")
	}
	tenantID := os.Getenv("AZURE_TENANT_ID")
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")

	client, err := azureclient.NewClient(tenantID, clientID, clientSecret)
	if err != nil {
		log.Fatal().Err(err).Msg("Error creating client")
	}
	client.IterateUsers(ctx, func(user *azureclient.User) bool {
		client.IterateUserGroups(ctx, user.ID, func(group string) bool {
			user.Groups = append(user.Groups, group)
			return true
		})
		log.Info().Interface("user", user).Msg("User")
		return true
	})
	client.IterateUsers(ctx, func(user *azureclient.User) bool {
		userID := user.ID
		user, err := client.GetUserById(ctx, userID)
		if err != nil {
			log.Error().Err(err).Msg("Error getting user")
			return true
		}
		log.Info().Interface("user", user).Msg("User")
		return true
	})
	client.IterateUsers(ctx, func(user *azureclient.User) bool {
		email := user.Mail
		user, err := client.GetUserByEmail(ctx, email)
		if err != nil {
			log.Error().Err(err).Msg("Error getting user")
			return true
		}
		log.Info().Interface("user", user).Msg("User")
		return true
	})
}
