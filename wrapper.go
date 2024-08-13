package main

import (
	"context"
	"fmt"

	"github.com/fiatjaf/khatru"
	"github.com/fiatjaf/relay29"
	"github.com/nbd-wtf/go-nostr"
	"golang.org/x/exp/slices"
)

func (w *Wrapper) QueryEvents(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
	hasRightKind := false

	for _, kind := range filter.Kinds {
		if kind == 30023 {
			hasRightKind = true
			break
		}
	}

	if w.state == nil || !hasRightKind {
		return w.Store.QueryEvents(ctx, filter)
	}

	pubkey := khatru.GetAuthed(ctx)
	userGroups := []string{}

	if pubkey != "" {
		state.Groups.Range(func(_ string, group *relay29.Group) bool {
			if _, isMember := group.Members[pubkey]; isMember {
				fmt.Println(pubkey, " is member of group", group)
				// add group to userGroups
				userGroups = append(userGroups, group.Address.ID)
				return true
			}

			return false
		})

		fmt.Println("userGroups", pubkey, userGroups)
	}

	retChannel := make(chan *nostr.Event, 500)

	queryChannel, err := w.Store.QueryEvents(ctx, filter)

	if err != nil {
		fmt.Println("error querying events", err)
	}

	go func() {
		defer close(retChannel)

		for event := range queryChannel {
			eventTiers := getTiersFromEvent(event)
			isPreview := len(event.Tags.GetAll([]string{"full", ""})) > 0
			userBelongsToATaggedGroup := false
			hTags := event.Tags.GetAll([]string{"h", ""})

			fmt.Println("Event id", event.ID, "isPreview", isPreview, eventTiers)

			// if it is not a preview and it doesnt have any tiers, send it
			if !isPreview && len(eventTiers) == 0 {
				fmt.Println("sending event id ", event.ID, "because it has no tiers and is not a preview")
				retChannel <- event
				continue
			}

			// go through all the htags of the event
			for _, tag := range hTags {
				fmt.Println("hTag in event", tag)

				// if the htag is in the user groups
				if slices.Contains(userGroups, tag[1]) {
					userBelongsToATaggedGroup = true
					break
				}
			}

			// if the user belongs to the tagged group
			if userBelongsToATaggedGroup {
				// if the event has tiers send it
				if len(eventTiers) > 0 {
					fmt.Println("sending event id ", event.ID, "because it has tiers and the user belongs to any of the tagged groups")
					retChannel <- event
					continue
				}
			} else {
				// if any of the event is a preview, send sit
				if isPreview {
					fmt.Println("sending event id ", event.ID, "because it is a preview and the user does not belong to any of the tagged group")
					retChannel <- event
					continue
				}
			}
		}
	}()

	return retChannel, nil
}
