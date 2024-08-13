package main

import (
	"fmt"

	"github.com/nbd-wtf/go-nostr"
)

/**
 * Does this event require a paid membership to view?
 */
func EventIsPaid(event nostr.Event) bool {
	eventTiers := getTiersFromEvent(&event)
	hasFreeTier := false

	if len(eventTiers) == 0 {
		return false
	}

	for _, tier := range eventTiers {
		if tier == "free" {
			hasFreeTier = true
			break
		}
	}

	fmt.Println("event", event.ID, "hasFreeTier", hasFreeTier, eventTiers)

	return !hasFreeTier
}

func getTiersFromEvent(event *nostr.Event) []string {
	fTags := event.Tags.GetAll([]string{"f", ""})
	eventTiers := make([]string, 0, len(fTags))
	for _, fTag := range fTags {
		eventTiers = append(eventTiers, fTag[1])
	}
	return eventTiers
}
