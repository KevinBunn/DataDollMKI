package main

import (
	db "GolandProjects/DataDollMKI/database"
	"GolandProjects/DataDollMKI/structs"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var defaultManagePermissions int64 = discordgo.PermissionManageServer
var (
	locationRoleIDs = []string{
		"872866532055736401",
		"872866704533905439",
		"872866670643929158",
		"872866524090728499",
		"872870817522909284",
		"872866719276875906",
		"872867303094644786",
		"872868521384738867",
	}
	classRoleIDs = []string{
		"1037403429800247317",
		"872704067607072838",
		"872702521196548166",
		"872703493960531988",
		"872705316230402088",
		"872704437632786493",
		"872704379310972940",
		"872705134759657482",
		"872704519400747019",
		"872704463285157959",
		"872703182294372402",
		"872704487846969414",
	}
	playStyleRoleIDs = []string{
		"1042287324177907762",
		"1042291456150355999",
		"1042471110899417208",
	}
)

var commands = []*discordgo.ApplicationCommand{
	{
		Name: "hello",
		// All commands and options must have a description
		// Commands/options without description will fail the registration
		// of the command.
		Description: "Hello World",
	},
	{
		Name:        "upload-event",
		Description: "Upload event pairing data to database",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "format",
				Description: "The format of the event (Classic Constructed, Blitz, etc)",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "type",
				Description: "The event type. (Armory, On Demand, etc)",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "lgs-name",
				Description: "The name of the LGS that hosted the event",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "date",
				Description: "Date of the event. Format: Mon DD, YYYY",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionAttachment,
				Name:        "pairings",
				Description: "the pairings.csv generated by Gem. Upload this file first",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionAttachment,
				Name:        "players",
				Description: "The players.csv file from Gem. Upload this file second",
				Required:    true,
			},
		},
	},
	{
		Name:        "link-gem-id",
		Description: "Used to map your discord profile to your gem profile",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "gem-id",
				Description: "Your Gem ID from fabtcg.com",
				Required:    true,
			},
		},
	},
	{
		Name:        "leaderboard",
		Description: "Show the highest gem xp of the server.",
	},
	{
		Name:                     "post-role-selections",
		Description:              "Use to have the bot post reaction selects",
		DefaultMemberPermissions: &defaultManagePermissions,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Choose the channel which you want the bot to post in.",
				Required:    true,
			},
		},
	},

	{
		Name:        "test-message",
		Description: "send a test message",
	},
}

var commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
	"hello": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Hello World!",
			},
		})
	},
	"upload-event": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		var (
			eventFormat          string
			eventType            string
			eventStoreName       string
			eventDate            string
			pairingsAttachmentId string
			playersAttachmentId  string
		)
		for _, option := range i.ApplicationCommandData().Options {
			switch option.Name {
			case "format":
				eventFormat = fmt.Sprint(option.Value)
			case "type":
				eventType = fmt.Sprint(option.Value)
			case "lgs-name":
				eventStoreName = fmt.Sprint(option.Value)
			case "date":
				eventDate = fmt.Sprint(option.Value)
			case "pairings":
				pairingsAttachmentId = fmt.Sprint(option.Value)
			case "players":
				playersAttachmentId = fmt.Sprint(option.Value)
			default:
			}
		}

		/**
		 * Download and parse the players and pairings files
		 **/
		pairingsAtt := i.ApplicationCommandData().Resolved.Attachments[pairingsAttachmentId]
		playersAtt := i.ApplicationCommandData().Resolved.Attachments[playersAttachmentId]

		pairingsResp, err := http.Get(pairingsAtt.URL)
		if err != nil {
			log.Println(err)
			errorRespond(s, i, "There was an error downloading pairings.csv")
		}
		defer pairingsResp.Body.Close()
		csvReader := csv.NewReader(pairingsResp.Body)
		pairingRecords, err := csvReader.ReadAll()
		if err != nil {
			log.Println(err)
			errorRespond(s, i, "There was an error reading pairings.csv")
		}
		// remove first element
		_, pairingRecords = pairingRecords[0], pairingRecords[1:]

		pairings := make([]structs.Pairing, 0)
		for _, val := range pairingRecords {
			pairings = append(pairings, structs.Pairing{
				Round:   val[0],
				Player1: val[3],
				Player2: val[5],
				Winner:  val[6],
			})
		}

		// Start Players csv
		playersResp, err := http.Get(playersAtt.URL)
		if err != nil {
			log.Println(err)
			errorRespond(s, i, "There was an error downloading players.csv")

		}
		defer playersResp.Body.Close()
		csvReader = csv.NewReader(playersResp.Body)
		playerRecords, err := csvReader.ReadAll()
		if err != nil {
			log.Println(err)
			errorRespond(s, i, "There was an error reading players.csv")
		}
		// remove first element
		_, playerRecords = playerRecords[0], playerRecords[1:]
		players := make([]structs.Player, 0)
		// prep gemID array for use later
		gemIDs := make([]string, len(playerRecords))
		for _, val := range playerRecords {
			if len(val) < 4 {
				errorRespond(s, i, "The hero's column was not added to players.csv. Please add the hero's column and run the command again")
			}
			players = append(players, structs.Player{
				Name:  val[1],
				GemID: val[2],
				Hero:  val[3],
			})
			gemIDs = append(gemIDs, val[2])
		}

		/**
		 * Create Event
		 */
		event := structs.Event{
			Type:         eventType,
			Format:       eventFormat,
			Participants: players,
			Pairings:     pairings,
			Date:         eventDate,
			Store:        eventStoreName,
		}
		err = db.UploadEvent(event)
		if err != nil {
			log.Println(err)
			errorRespond(s, i, "Could not upload event to the database")
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Successfully uploaded event.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})

		messageUsersForCommunityVote(s, i, players, gemIDs, event.Store, event.Date)
	},
	"link-gem-id": func(s *discordgo.Session, i *discordgo.InteractionCreate) {

		gemID := fmt.Sprint(i.ApplicationCommandData().Options[0].Value)
		err := db.LinkGemID(gemID, i.Member.User.ID)
		if err != nil {
			log.Println(err)
			errorRespond(s, i, "There was an error uploading your ID")
		}

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Your Gem ID is linked.",
			},
		})
	},
	"leaderboard": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		var embedFields []*discordgo.MessageEmbedField
		discordToGemMap, err := db.GetDiscordToGemMap()
		if err != nil {
			log.Println(err)
			errorRespond(s, i, "Could not get player to discord map.")
		}
		pairings, err := db.GetEventPairings()
		if err != nil {
			log.Println(err)
			errorRespond(s, i, "Could not get player pairings.")
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				// Content: "Getting pairing data",
			},
		})
		for _, member := range discordToGemMap {
			var discordID = fmt.Sprint(member["discordID"])
			var gemID = fmt.Sprint(member["gemID"])
			var winCount = 0
			for _, pair := range pairings {
				if pair.Player1 == gemID && pair.Winner == "1WIN" {
					winCount++
				} else if pair.Player2 == gemID && pair.Winner == "2WIN" {
					winCount++
				}
			}
			// find discord member
			guildMember, err := s.GuildMember(i.GuildID, discordID)
			if err != nil {
				log.Println(err)
				sendSimpleMessage(s, i, "Could not find member with ID: "+discordID)
			}
			xp := winCount * 3
			var name string
			// check if the user has a nickname, use username if not
			if guildMember.Nick == "" {
				name = guildMember.User.Username
			} else {
				name = guildMember.Nick
			}
			embedFields = append(embedFields, &discordgo.MessageEmbedField{Name: name, Value: strconv.Itoa(xp), Inline: false})
		}
		// sort by highest value desc
		sort.Slice(embedFields, func(i, j int) bool {
			a, _ := strconv.Atoi(embedFields[i].Value)
			b, _ := strconv.Atoi(embedFields[j].Value)
			return a > b
		})
		// add an integer for leaderboard position after sort
		for i, field := range embedFields {
			field.Name = fmt.Sprint(i+1) + ". " + field.Name
		}

		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:  "XP Leaderboard",
					Color:  11342935,
					Fields: embedFields,
					Footer: &discordgo.MessageEmbedFooter{
						Text: time.Now().Format("Jan 2, 2006"),
					},
				},
			},
		})
	},
	"post-role-selections": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		channelID := fmt.Sprint(i.ApplicationCommandData().Options[0].Value)
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Posting select components",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		// Location Select
		_, err = s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:       "What City Do You Live In?",
					Color:       38144,
					Description: "Assign yourself a role to help coordinate with others.",
					Image: &discordgo.MessageEmbedImage{
						URL: "https://i.imgur.com/oYSuQQy.jpg",
					},
				},
			},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							CustomID:    "location-select",
							Placeholder: "Make a selection",
							Options: []discordgo.SelectMenuOption{
								{
									Label:   "Boise",
									Value:   "872866532055736401",
									Default: false,
								},
								{
									Label: "Eagle",
									Value: "872866704533905439",
								},
								{
									Label: "Meridian",
									Value: "872866670643929158",
								},
								{
									Label: "Nampa",
									Value: "872866524090728499",
								},
								{
									Label: "Caldwell",
									Value: "872870817522909284",
								},
								{
									Label: "Kuna",
									Value: "872866719276875906",
								},
								{
									Label: "Other City in Valley",
									Value: "872867303094644786",
								},
								{
									Label: "Outsider",
									Value: "872868521384738867",
								},
							},
						},
					},
				},
			},
		})
		// Class Select
		_, err = s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{
				{
					Title: "Which Class Do You Like To Play Most?",
					Color: 14988637,
					Description: "These are to let others know what you like to play, or you could just pick the color you like most for cosmetics." +
						"\n\n<:arakni:1037406267020410890> <@&1037403429800247317>\n<:rhinar:872874589414371398> <@&872704067607072838>\n<:bravo:872874739755020418> <@&872702521196548166>\n<:prism:872874695932928040> <@&872703493960531988>\n<:dash:872874764769833045> <@&872705316230402088>\n<:kavdaen:872874833153777664> <@&872704437632786493>\n<:katsu:872874854582456350> <@&872704379310972940>\n<:azalea:872874728442966067> <@&872705134759657482>\n<:viserai:872874916825927690> <@&872704519400747019>\n<:shiyana:872874893748867184> <@&872704463285157959>\n<:dorinthea:872874779693158431> <@&872703182294372402>\n<:kano:872874809091031060> <@&872704487846969414>",
				},
			},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							CustomID:    "class-select",
							Placeholder: "Make a selection",
							Options: []discordgo.SelectMenuOption{
								{
									Label:   "Assassin",
									Value:   "1037403429800247317",
									Default: false,
								},
								{
									Label: "Brute",
									Value: "872704067607072838",
								},
								{
									Label: "Guardian",
									Value: "872702521196548166",
								},
								{
									Label: "Illusionist",
									Value: "872703493960531988",
								},
								{
									Label: "Mechanologist",
									Value: "872705316230402088",
								},
								{
									Label: "Merchant",
									Value: "872704437632786493",
								},
								{
									Label: "Ninja",
									Value: "872704379310972940",
								},
								{
									Label: "Ranger",
									Value: "872705134759657482",
								},
								{
									Label: "Runeblade",
									Value: "872704519400747019",
								},
								{
									Label: "Shapeshifter",
									Value: "872704463285157959",
								},
								{
									Label: "Warrior",
									Value: "872703182294372402",
								},
								{
									Label: "Wizard",
									Value: "872704487846969414",
								},
							},
						},
					},
				},
			},
		})
		// Play type select
		_, err = s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:       "Play Notifications",
					Color:       8860200,
					Description: "Which of these play styles are you interesting in getting notifications for?",
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:   "Armory",
							Value:  "Get notifications on when you are able to sign up for an Armory.",
							Inline: false,
						},
						{
							Name:   "Casual Play",
							Value:  "Get notifications from others looking to join up for causal play.",
							Inline: false,
						},
						{
							Name:   "Online Play",
							Value:  "Get notifications from others looking to play online via webcam, talishar.net, etc.",
							Inline: false,
						},
					},
				},
			},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							CustomID:    "play-style-select",
							Placeholder: "Make selections",
							MaxValues:   3,
							Options: []discordgo.SelectMenuOption{
								{
									Label:   "Armory",
									Value:   "1042287324177907762",
									Default: false,
								},
								{
									Label: "Casual Play",
									Value: "1042291456150355999",
								},
								{
									Label: "Online Play",
									Value: "1042471110899417208",
								},
							},
						},
					},
				},
			},
		})
		fmt.Println(err)
	},
	"test-message": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		// discordToGemMap, _ := db.GetDiscordToGemMap()
		err, voters := db.GetLatest4WeeksVotingForStore("Your Toy Link")
		if err != nil {
			fmt.Println(err)
			errorRespond(s, i, "Error getting voting data")
		}

		// display them in a discordgo.MessageEmbed
		embed := &discordgo.MessageEmbed{
			Title:       "Voting Data",
			Color:       8860200,
			Description: "Here is the voting data for the last 4 weeks",
			Fields:      []*discordgo.MessageEmbedField{},
		}
		guildMembersChunk := s.RequestGuildMembers(i.GuildID, "", 0, "", false)
		fmt.Print(guildMembersChunk)
		for _, voter := range voters {
			// var voterName = ""
			// for _, guild := range guildMembersChunk {
			// 	if guild.ID == i.GuildID {
			// 		for _, member := range guild.Members {
			// 			if member.User.ID == voter.DiscordID {
			// 				voterName = member.User.Username
			// 			}
			// 		}
			// 	}
			// }

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   voter.DiscordID,
				Value:  fmt.Sprintf("%d", voter.Recieved),
				Inline: true,
			})
		}
		// respond to the interaction
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Posting ",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})

		_, err = s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{
				embed,
			},
		})

	},
}
var componentsHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
	"location-select": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		// Give list of roles to user
		errString := handleSelectInteraction(s, i, locationRoleIDs)
		if errString != "" {
			errorRespond(s, i, errString)
		}
	},
	"class-select": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		// Give list of roles to user
		errString := handleSelectInteraction(s, i, classRoleIDs)
		if errString != "" {
			errorRespond(s, i, errString)
		}
	},
	"play-style-select": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		errString := handleSelectInteraction(s, i, playStyleRoleIDs)
		if errString != "" {
			errorRespond(s, i, errString)
		}
	},
	"community-vote-submit": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		// loc, _ := time.LoadLocation("America/Denver")
		// fmt.Println(i.User.Username + " voted")
		// fmt.Println(i.Message.Timestamp.In(loc).Date())
		embedFields := i.Message.Embeds[0].Fields
		storeNameField := findEmbedFieldByName(embedFields, "Store Name")
		if storeNameField != nil {
			fmt.Println(storeNameField.Value)
		}
		dateField := findEmbedFieldByName(embedFields, "Date")
		if dateField != nil {
			fmt.Println(dateField.Value)
		}

		err := db.UpdateVote(storeNameField.Value, dateField.Value, i.User.ID, i.MessageComponentData().Values[0])
		if err != nil {
			fmt.Println(err)
			errorRespond(s, i, "Could not update your vote. Please let Strezzkev know.")
		} else {
			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content:    "Thank you, your vote has been recorded",
					Components: []discordgo.MessageComponent{},
				},
			})
			if err != nil {
				sendSimpleMessage(s, i, "The interaction failed to respond, you should probably tell Kevin")
			}
		}

		// channel, err := s.UserChannelCreate("453428099544252417")
		channel, err := s.UserChannelCreate("241699487972589570")
		// if err != nil {
		// 	sendSimpleMessage(s, i, "Oh no, I couldn't update the vote count, please tell Sam your vote instead")
		// }
		//instead of sending a message, we need to update the database
		s.ChannelMessageSend(channel.ID, i.User.Username+" voted for "+i.MessageComponentData().Values[0]+", "+dateField.Value)
	},
}

func findEmbedFieldByName(embedFields []*discordgo.MessageEmbedField, name string) *discordgo.MessageEmbedField {
	for _, field := range embedFields {
		if field.Name == name {
			return field
		}
	}
	return nil
}

func RemoveIndex[T any](s []T, index int) []T {
	ret := make([]T, 0)
	ret = append(ret, s[:index]...)
	return append(ret, s[index+1:]...)
}

func messageUsersForCommunityVote(s *discordgo.Session, i *discordgo.InteractionCreate, players []structs.Player, gemIDs []string, storeName string, date string) {
	gemToDiscordMap, err := db.GetGemToDiscordMapByGemIDs(gemIDs)
	if err != nil {
		fmt.Println(err)
		// sendSimpleMessage(s, i, "Could not get the discord to gem map")
	}
	for index, player := range players {
		var playerChoices []structs.Player
		var discordID string
		discordID = gemToDiscordMap[player.GemID]
		// for _, entry := range gemToDiscordMap {
		// 	if player.GemID == entry["gemID"] {
		// 		discordID = entry["discordID"].(string)
		// 	}
		// }
		playerChoices = RemoveIndex(players, index)

		fmt.Println(playerChoices)
		if discordID == "" {
			// we don't know the discord ID so it will have to be done manually.
			fmt.Println(player.Name + " is not in firebase")
			// sendSimpleMessage(s, i, player.Name+" is not in firebase")
		} else {
			// message the user to vote for a list of players that does not include them.
			sendVoteMessage(s, i, discordID, playerChoices, gemToDiscordMap, player.Name, storeName, date)
		}
	}
}

func sendVoteMessage(s *discordgo.Session, i *discordgo.InteractionCreate, discordID string, playerChoices []structs.Player, gemToDiscordMap map[string]string, playerName string, storeName string, date string) {
	channel, err := s.UserChannelCreate(discordID)
	if err != nil {
		errorRespond(s, i, "Could not get the channel for the user.")
	}
	// Create vote options
	votingOptions := make([]discordgo.SelectMenuOption, len(playerChoices))
	for index, choice := range playerChoices {
		votingOptions[index] = discordgo.SelectMenuOption{
			Label: choice.Name,
			Value: gemToDiscordMap[choice.GemID],
		}
	}
	s.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{
			{
				Title:       "Vote for Community Mat",
				Color:       8860200,
				Description: "Thanks for coming to the Armory " + strings.Split(playerName, " ")[0] + "! Select one of the other attendees to cast your vote for who should recieve the community mat.",
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:  "Store Name",
						Value: storeName,
					},
					{
						Name:  "Date",
						Value: date,
					},
				},
			},
		},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    "community-vote-submit",
						Placeholder: "Select Attendee",
						Options:     votingOptions,
					},
				},
			},
		},
	})
}

func handleSelectInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, applicableRoleIDs []string) string {
	userId := i.Member.User.ID
	selections := i.MessageComponentData()

	// Search for any previously selected roles
	for _, role := range i.Member.Roles {
		for _, oldRoleID := range applicableRoleIDs {
			if role == oldRoleID {
				err := s.GuildMemberRoleRemove(os.Getenv("GUILD_ID"), userId, role)
				if err != nil {
					fmt.Println(err)
					return "Could not remove old role"
				}
			}
		}
	}
	// Assign newly selected roles
	for _, selection := range selections.Values {
		err := s.GuildMemberRoleAdd(os.Getenv("GUILD_ID"), userId, selection)
		if err != nil {
			fmt.Println(err)
			return "Could not add role"
		}
	}
	// Obligatory response
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		// do nothing.
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
	if err != nil {
		fmt.Println(err)
		return "The interaction failed to respond, but you should still have the new role(s)"
	}
	return ""
}

func sendSimpleMessage(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.ChannelMessageSend(i.ChannelID, message)
}

func errorRespond(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func startServer() *discordgo.Session {
	discord, _ := discordgo.New("Bot " + os.Getenv("TOKEN"))

	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		cmd, err := discord.ApplicationCommandCreate(os.Getenv("CLIENT_ID"), os.Getenv("GUILD_ID"), v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}

	discord.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		fmt.Println("Bot is ready")
	})
	// discord.AddHandler()
	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
				h(s, i)
			}
		case discordgo.InteractionMessageComponent:
			if h, ok := componentsHandlers[i.MessageComponentData().CustomID]; ok {
				h(s, i)
			}
		}
	})

	err := discord.Open()
	if err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}

	return discord
}

func main() {
	if os.Getenv("ENV") != "Production" {
		err := godotenv.Load()
		if err != nil {
			log.Fatalf("Error loading .env file")
		}
	}

	db.StartFireBase()
	discord := startServer()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	cmds, err := discord.ApplicationCommands(os.Getenv("CLIENT_ID"), os.Getenv("GUILD_ID"))
	if err != nil {
		log.Println("Could not retrieve registered commands", err)
	}
	for _, cmd := range cmds {
		err = discord.ApplicationCommandDelete(os.Getenv("CLIENT_ID"), os.Getenv("GUILD_ID"), cmd.ID)
		if err != nil {
			log.Println("Could not delete registered command: "+cmd.Name, err)
		}
	}

	discord.Close()
	db.CloseFireBase()
}
