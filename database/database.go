package database

import (
	"GolandProjects/DataDollMKI/structs"
	"cloud.google.com/go/firestore"
	"context"
	firebase "firebase.google.com/go"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"log"
	"time"
)

// Global variables that will be accessed in most/all functions
var ctx = context.Background()
var client *firestore.Client

// StartFireBase Initialize Firebase connection
func StartFireBase() {
	serviceAccount := option.WithCredentialsFile("./google-credentials.json")
	app, err := firebase.NewApp(ctx, nil, serviceAccount)
	if err != nil {
		log.Fatalln(err)
	}
	client, err = app.Firestore(ctx)
	if err != nil {
		log.Fatalln(err)
	}
}

func CloseFireBase() {
	err := client.Close()
	if err != nil {
		log.Fatalln(err)
	}
}

func UploadEvent(event structs.Event) error {
	_, _, err := client.Collection("events").Add(ctx, map[string]interface{}{
		"format":       event.Format,
		"type":         event.Type,
		"store":        event.Store,
		"participants": event.Participants,
		"pairings":     event.Pairings,
		"date":         time.Now().Format("Jan 2, 2006"),
	})
	return err
}

func LinkGemID(gemID string, userID string) error {
	_, _, err := client.Collection("discord-to-gem").Add(ctx, map[string]interface{}{
		"gemID":     gemID,
		"discordID": userID,
	})
	return err
}

func GetDiscordToGemMap() ([]map[string]interface{}, error) {
	var playerMap []map[string]interface{}
	iter := client.Collection("discord-to-gem").Documents(ctx)
	for {
		snapshot, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		playerMap = append(playerMap, snapshot.Data())
	}
	return playerMap, nil
}

func GetEventPairings() ([]structs.Pairing, error) {
	var pairings []structs.Pairing
	iter := client.Collection("events").Documents(ctx)
	for {
		snapshot, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var tempEvent structs.Event
		err = snapshot.DataTo(&tempEvent)
		if err != nil {
			return nil, err
		}
		pairings = append(pairings, tempEvent.Pairings...)
	}

	return pairings, nil
}
