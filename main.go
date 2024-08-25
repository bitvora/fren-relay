package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/fiatjaf/eventstore/postgresql"
	"github.com/fiatjaf/khatru"
	"github.com/joho/godotenv"
	"github.com/nbd-wtf/go-nostr"
)

type Config struct {
	RelayName        string
	OwnerPubkey      string
	RelayDescription string
	FetchRelay       string
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
	relay.Info.PubKey = config.OwnerPubkey
	relay.Info.Description = config.RelayDescription

	db := postgresql.PostgresBackend{
		DatabaseURL: "postgres://" + config.PostgresUser + ":" + config.PostgresPassword + "@" + config.PostgresHost + ":" + config.PostgresPort + "/" + config.PostgresDB + "?sslmode=disable",
	}
	if err := db.Init(); err != nil {
		panic(err)
	}

	frens := getFollowedPubkeys()
	fmt.Println("allowed frens: ", len(frens))

	relay.OnConnect = append(relay.OnConnect, func(ctx context.Context) {
		khatru.RequestAuth(ctx)
	})

	relay.RejectFilter = append(relay.RejectFilter, func(ctx context.Context, filter nostr.Filter) (bool, string) {

		authenticatedUser := khatru.GetAuthed(ctx)
		for _, fren := range frens {
			if authenticatedUser == fren {
				return false, ""
			}
		}

		return true, "auth-required: this query requires you to be authenticated"
	})

	relay.RejectEvent = append(relay.RejectEvent, func(ctx context.Context, event *nostr.Event) (bool, string) {

		authenticatedUser := khatru.GetAuthed(ctx)
		for _, fren := range frens {
			if authenticatedUser == fren {
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

func getFollowedPubkeys() []string {
	ctx := context.Background()
	config := LoadConfig()

	relay, err := nostr.RelayConnect(ctx, config.FetchRelay)
	if err != nil {
		log.Fatalf("Failed to connect to relay: %s", err)
	}

	var filters nostr.Filters
	filters = []nostr.Filter{{
		Kinds:   []int{nostr.KindContactList},
		Authors: []string{config.OwnerPubkey},
		Limit:   1,
	}}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	sub, err := relay.Subscribe(ctx, filters)
	if err != nil {
		panic(err)
	}

	var pubkeys []string

	for ev := range sub.Events {
		follows := ev.Tags.GetAll([]string{"p"})
		for _, follow := range follows {
			pubkeys = append(pubkeys, follow[1])
		}
	}

	return pubkeys
}

func LoadConfig() Config {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	config := Config{
		RelayName:        getEnv("RELAY_NAME"),
		OwnerPubkey:      getEnv("OWNER_PUBKEY"),
		RelayDescription: getEnv("RELAY_DESCRIPTION"),
		FetchRelay:       getEnv("FETCH_RELAY"),
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
