package main

import (
	"context"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
)

func rejectCreatingExistingGroups(ctx context.Context, event *nostr.Event) (bool, string) {
	// check if the group exists
	if event.Kind == 9007 {
		err := checkGroupExists(ctx, event)
		if err != nil {
			return true, err.Error()
		}
	}

	return false, ""
}

func checkGroupExists(ctx context.Context, event *nostr.Event) error {
	// check if the group exists
	group := state.GetGroupFromEvent(event)

	if group != nil {
		return fmt.Errorf("group already exists")
	}

	return nil
}
