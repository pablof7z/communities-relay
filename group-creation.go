package main

import (
	"context"

	"github.com/nbd-wtf/go-nostr"
)

func createGroups(ctx context.Context, event *nostr.Event) (reject bool, msg string) {
	// if event.Kind == 9007 {
	// 	return true, "create a group"
	// }

	return false, ""
}
