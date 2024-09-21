package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/fiatjaf/eventstore"
	"github.com/fiatjaf/eventstore/lmdb"
	"github.com/fiatjaf/khatru"
	"github.com/fiatjaf/khatru/policies"
	"github.com/fiatjaf/relay29"
	"github.com/fiatjaf/relay29/khatru29"
	"github.com/kelseyhightower/envconfig"
	"github.com/nbd-wtf/go-nostr"
	"github.com/rs/zerolog"
	"golang.org/x/exp/slices"
)

type Settings struct {
	Port             string `envconfig:"PORT" default:"5577"`
	Domain           string `envconfig:"DOMAIN" required:"true"`
	RelayName        string `envconfig:"RELAY_NAME" required:"true"`
	RelayPrivkey     string `envconfig:"RELAY_PRIVKEY" required:"true"`
	RelayDescription string `envconfig:"RELAY_DESCRIPTION"`
	RelayContact     string `envconfig:"RELAY_CONTACT"`
	RelayIcon        string `envconfig:"RELAY_ICON"`
	DatabasePath     string `envconfig:"DATABASE_PATH" default:"./db"`

	RelayPubkey string `envconfig:"-"`
}

var (
	s     Settings
	db    = &lmdb.LMDBBackend{}
	log   = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	relay *khatru.Relay
	state *relay29.State

	dbWrapper = &Wrapper{db, nil, nil}
)

type Wrapper struct {
	eventstore.Store
	state *relay29.State

	realQueryEvents func(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error)
}

var _ eventstore.Store = (*Wrapper)(nil)

func main() {
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't process envconfig")
		return
	}
	s.RelayPubkey, _ = nostr.GetPublicKey(s.RelayPrivkey)

	// load db
	db.Path = s.DatabasePath
	if err := db.Init(); err != nil {
		log.Fatal().Err(err).Msg("failed to initialize database")
		return
	}
	log.Debug().Str("path", db.Path).Msg("initialized database")

	wrappedDb := &Wrapper{db, nil, db.QueryEvents}

	relay, state = khatru29.Init(relay29.Options{
		Domain:    "localhost:" + s.Port,
		DB:        wrappedDb,
		SecretKey: s.RelayPrivkey,
	})
	wrappedDb.state = state

	// init relay
	relay.Info.Name = "Communities relay"
	relay.Info.PubKey, _ = nostr.GetPublicKey(s.RelayPrivkey)
	relay.Info.Description = "A relay for communities"
	relay.Info.SupportedNIPs = append(relay.Info.SupportedNIPs, 29)

	relay.StoreEvent = append(relay.StoreEvent, db.SaveEvent)
	relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)

	relay.QueryEvents = append(relay.QueryEvents, state.NormalEventQuery)

	// extra policies
	relay.RejectEvent = slices.Insert(
		relay.RejectEvent, 0,
		policies.PreventTimestampsInThePast(60*time.Second),
		policies.PreventTimestampsInTheFuture(30*time.Second),
		rejectCreatingExistingGroups,
	)

	publicStore := db

	publicRelay := khatru.NewRelay()
	publicRelay.StoreEvent = append(publicRelay.StoreEvent, publicStore.SaveEvent, func(ctx context.Context, event *nostr.Event) error {
		fmt.Println("storing event", event.ID)

		return nil
	})
	publicRelay.QueryEvents = append(publicRelay.QueryEvents, func(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
		retChannel := make(chan *nostr.Event, 500)

		storeChannel, err := publicStore.QueryEvents(ctx, filter)

		if err != nil {
			return nil, err
		}

		go func() {
			defer close(retChannel)

			for event := range storeChannel {
				if EventIsPaid(*event) == false {
					fmt.Println("sending event id ", event.ID, "because it is not paid")
					retChannel <- event
				}
			}
		}()

		return retChannel, nil
	})
	publicRelay.CountEvents = append(publicRelay.CountEvents, publicStore.CountEvents)
	publicRelay.DeleteEvent = append(publicRelay.DeleteEvent, publicStore.DeleteEvent)

	router := khatru.NewRouter()
	router.Info.SupportedNIPs = append(relay.Info.SupportedNIPs, 29)

	router.Route().
		Req(func(filter nostr.Filter) bool {
			_, hasHTag := filter.Tags["h"]
			if hasHTag {
				fmt.Println("REQ", filter, "going to NIP-29 relay")
				return true
			}
			// if the filter.kinds includes 39000, 39001 or 39002 return true
			for _, kind := range filter.Kinds {
				if kind >= 39000 && kind <= 39002 {
					fmt.Println("REQ", filter, "going to NIP-29 relay")
					return true
				}
			}

			fmt.Println("REQ", filter, "NOT going to NIP-29 relay")
			return false
		}).
		Event(func(event *nostr.Event) bool {
			switch {
			case event.Kind <= 9021 && event.Kind >= 9000:
				fmt.Println("EVENT", event.Kind, "in NIP-29 relay")
				return true
			case event.Kind <= 39010 && event.Kind >= 39000:
				fmt.Println("EVENT", event.Kind, "in NIP-29 relay")
				return true
			case event.Kind <= 12 && event.Kind >= 9:
				fmt.Println("EVENT", event.Kind, "in NIP-29 relay")
				return true
			case event.Tags.GetFirst([]string{"h", ""}) != nil:
				fmt.Println("EVENT", event.Kind, "in NIP-29 relay")
				return true
			default:
				fmt.Println("EVENT", event.Kind, "NOT stored in NIP-29 relay")
				return false
			}
		}).
		Relay(relay)

	router.Route().
		Req(func(filter nostr.Filter) bool { return true }).
		Event(func(event *nostr.Event) bool { return true }).
		Relay(publicRelay)

	router.OnConnect = append(router.OnConnect, func(ctx context.Context) {
		khatru.RequestAuth(ctx)
	})

	// http routes
	router.Router().HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "nothing to see here, you must use a nip-29 powered client")
	})

	fmt.Println("running on http://0.0.0.0:" + s.Port)
	if err := http.ListenAndServe(":"+s.Port, router); err != nil {
		log.Fatal().Err(err).Msg("failed to serve")
	}
}
