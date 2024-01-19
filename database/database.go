package database

import (
	"GolandProjects/DataDollMKI/structs"
	"context"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		"date":         event.Date,
	})
	return err
}

func UpdateVote(storeName string, date string, voterId string, voteeId string) error {
	// TODO: handle if store document doesn't exist
	docRef := client.Collection("community-votes").Doc(storeName)

	snapshot, err := docRef.Get(ctx)
	fmt.Println(snapshot.Exists())
	if err != nil && status.Code(err) != codes.NotFound {
		return err // Handle errors other than notFound
	}

	var communityVotes map[string]map[string]interface{}
	if snapshot.Exists() {
		err = snapshot.DataTo(&communityVotes)
		if err != nil {
			return err
		}
	} else {
		// create an empty map
		communityVotes = make(map[string]map[string]interface{})
	}
	// First, check for date
	_, dateExists := communityVotes[date]
	if !dateExists {
		communityVotes[date] = make(map[string]interface{})
	}

	voterInterface, voterExists := communityVotes[date][voterId]
	voteeInterface, voteeExists := communityVotes[date][voteeId]
	if !voterExists {
		// set the initial vote
		var votes = make(map[string]interface{})
		votes["votesMade"] = 1
		votes["votesReceived"] = 0
		communityVotes[date][voterId] = votes
	} else {
		// cast interface into the correct type
		voterVoteCount, ok := voterInterface.(map[string]interface{})
		if !ok {
			// handle the case where the type assertion fails
			return fmt.Errorf("failed to cast voter vote count to map[string]interface{}")
		}
		voterVotesMade, ok := voterVoteCount["votesMade"].(int64)
		if !ok {
			// handle the case where the type assertion fails
			return fmt.Errorf("failed to cast voter votes made to int")
		}
		voterVotesMade++
		voterVoteCount["votesMade"] = voterVotesMade
		communityVotes[date][voterId] = voterVoteCount
	}

	if !voteeExists {
		var votes = make(map[string]interface{})
		votes["votesMade"] = 0
		votes["votesReceived"] = 1
		communityVotes[date][voteeId] = votes
	} else {
		voteeVoteCount, ok := voteeInterface.(map[string]interface{})
		if !ok {
			return fmt.Errorf("failed to cast votee vote count to map[string]interface{}")
		}
		voteeVotesReceived, ok := voteeVoteCount["votesReceived"].(int64)
		if !ok {
			return fmt.Errorf("failed to cast votee votes made to int")
		}
		voteeVotesReceived++
		voteeVoteCount["votesReceived"] = voteeVotesReceived
		communityVotes[date][voteeId] = voteeVoteCount
	}

	_, err = docRef.Set(ctx, communityVotes, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
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

func GetGemToDiscordMapByGemIDs(gemIDs []string) (map[string]string, error) {
	// create a query that filters on the "gemID" field
	var userMap = make(map[string]string)
	for _, gemID := range gemIDs {
		query := client.Collection("users").Where("gemID", "==", gemID)

		// retrieve the matching documents
		docs, err := query.Documents(ctx).GetAll()
		if err != nil {
			return nil, err
		}

		// iterate over the documents and do something with them
		for _, doc := range docs {
			// do something with the document

			userMap[doc.Data()["gemID"].(string)] = fmt.Sprint(doc.Ref.ID)
			// fmt.Println(doc.Ref.ID)
		}
	}

	return userMap, nil
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

func GetLatest4WeeksVotingForStore(storeName string) (error, []structs.Voter) {
	docRef := client.Collection("community-votes").Doc(storeName)
	snapshot, err := docRef.Get(ctx)
	// fmt.Println(snapshot.Exists())
	if err != nil && status.Code(err) != codes.NotFound {
		return err, nil // Handle errors other than notFound
	}

	var communityVotes map[string]map[string]interface{}
	err = snapshot.DataTo(&communityVotes)
	if err != nil {
		log.Fatalf("Failed to parse document data: %v", err)
		return err, nil
	}
	votersMap := make(map[string]map[string]int)
	// Iterate over the dates in the community votes and get the latest 4 weeks
	for date, votes := range communityVotes {
		// Parse the date to check if it is in the last 4 weeks
		date = strings.Replace(date, "Sept", "Sep", 1)

		t, err := time.Parse("Jan 2, 2006", date)
		if err != nil {
			log.Fatalf("Failed to parse date: %v", err)
			return err, nil
		}
		if t.After(time.Now().AddDate(0, 0, -28)) {
			for voterId, voteData := range votes {
				// add voterID to the voters map if it doesn't exist
				_, exists := votersMap[voterId]
				if !exists {
					votersMap[voterId] = make(map[string]int)
				}
				// increment the votes made and votes received
				votersMap[voterId]["votesMade"] += int(voteData.(map[string]interface{})["votesMade"].(int64))
				votersMap[voterId]["votesReceived"] += int(voteData.(map[string]interface{})["votesReceived"].(int64))
			}
		}
	}
	var sorted []structs.Voter
	for k, v := range votersMap {
		sorted = append(sorted, structs.Voter{DiscordID: k, Recieved: v["votesReceived"], Made: v["votesMade"]})
	}
	// sort by votes received then by votes made
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Recieved == sorted[j].Recieved {
			// if made is the same then pick a random one
			if sorted[i].Made == sorted[j].Made {
				return rand.Intn(2) == 0
			}
			return sorted[i].Made > sorted[j].Made
		}
		return sorted[i].Recieved > sorted[j].Recieved
	})

	// print the top 10 voters
	// for i := 0; i < 10; i++ {
	// 	fmt.Println(sorted[i].GemID, sorted[i].Recieved, sorted[i].Made)
	// }

	return nil, sorted
}
