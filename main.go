package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/fiatjaf/eventstore/postgresql"
	"github.com/fiatjaf/khatru"
	"github.com/joho/godotenv"
	"github.com/nbd-wtf/go-nostr"
)

type Config struct {
	RelayName        string
	RelayPubkey      string
	RelayDescription string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresHost     string
	PostgresPort     string
}

type NostrData struct {
	Names  map[string]string   `json:"names"`
	Relays map[string][]string `json:"relays"`
}

var data NostrData

type Fren struct {
	Username string `json:"username"`
	PubKey   string `json:"pubkey"`
}

type FrenList struct {
	Frens []Fren `json:"frens"`
}

func main() {
	relay := khatru.NewRelay()

	config := LoadConfig()

	relay.Info.Name = config.RelayName
	relay.Info.PubKey = config.RelayPubkey
	relay.Info.Description = config.RelayDescription

	db := postgresql.PostgresBackend{
		DatabaseURL: "postgres://" + config.PostgresUser + ":" + config.PostgresPassword + "@" + config.PostgresHost + ":" + config.PostgresPort + "/" + config.PostgresDB + "?sslmode=disable",
	}
	if err := db.Init(); err != nil {
		panic(err)
	}

	frenFile, err := os.ReadFile("users.json")
	if err != nil {
		log.Fatalf("Failed to read JSON file: %s", err)
	}

	// Unmarshal the JSON file into the struct
	var frenList FrenList
	err = json.Unmarshal(frenFile, &frenList)
	if err != nil {
		log.Fatalf("Failed to parse JSON: %s", err)
	}

	// Access the map of usernames to public keys
	frens := frenList.Frens

	relay.OnConnect = append(relay.OnConnect, func(ctx context.Context) {
		khatru.RequestAuth(ctx)
	})

	relay.RejectFilter = append(relay.RejectFilter, func(ctx context.Context, filter nostr.Filter) (bool, string) {

		authenticatedUser := khatru.GetAuthed(ctx)
		for _, fren := range frens {
			if authenticatedUser == fren.PubKey {
				return false, ""
			}
		}

		return true, "auth-required: this query requires you to be authenticated"
	})

	relay.RejectEvent = append(relay.RejectEvent, func(ctx context.Context, event *nostr.Event) (bool, string) {

		authenticatedUser := khatru.GetAuthed(ctx)
		for _, fren := range frens {
			if authenticatedUser == fren.PubKey {
				return false, ""
			}
		}
		return true, "auth-required: publishing this event requires authentication"
	})

	relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent)
	relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)

	fmt.Println("running on :3334")
	http.ListenAndServe(":3334", relay)
}

func fetchNostrData(teamDomain string) {
	response, err := http.Get("https://" + teamDomain + "/.well-known/nostr.json")
	if err != nil {
		log.Printf("Error getting well known file: %v", err)
		return
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return
	}

	var newData NostrData
	err = json.Unmarshal(body, &newData)
	if err != nil {
		log.Printf("Error unmarshalling JSON: %v", err)
		return
	}

	data = newData
	log.Println("Updated NostrData from .well-known file")
}

func LoadConfig() Config {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	config := Config{
		RelayName:        getEnv("RELAY_NAME"),
		RelayPubkey:      getEnv("RELAY_PUBKEY"),
		RelayDescription: getEnv("RELAY_DESCRIPTION"),
		PostgresUser:     getEnv("POSTGRES_USER"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD"),
		PostgresDB:       getEnv("POSTGRES_DB"),
		PostgresHost:     getEnv("POSTGRES_HOST"),
		PostgresPort:     getEnv("POSTGRES_PORT"),
	}

	return config
}

func getEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		log.Fatalf("Environment variable %s not set", key)
	}
	return value
}
