package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/fiatjaf/eventstore/bolt"
	"github.com/fiatjaf/khatru"
	"github.com/fiatjaf/khatru/policies"
	"github.com/fiatjaf/relay29"
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
	db    = &bolt.BoltBackend{}
	log   = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	state *relay29.State
)

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

	state = relay29.Init(relay29.Options{
		Domain:    "localhost:" + s.Port,
		DB:        db,
		SecretKey: s.RelayPrivkey,
	})

	// init relay
	state.Relay.Info.Name = "Communities relay"
	state.Relay.Info.PubKey, _ = nostr.GetPublicKey(s.RelayPrivkey)
	state.Relay.Info.Description = "A relay for communities"
	state.Relay.Info.SupportedNIPs = append(state.Relay.Info.SupportedNIPs, 29)

	state.Relay.StoreEvent = append(state.Relay.StoreEvent, db.SaveEvent)
	state.Relay.DeleteEvent = append(state.Relay.DeleteEvent, db.DeleteEvent)

	// extra policies
	state.Relay.RejectEvent = slices.Insert(state.Relay.RejectEvent, 0,
		policies.PreventLargeTags(64),
		policies.PreventTooManyIndexableTags(20, []int{9005}, nil),
		policies.PreventTimestampsInThePast(60),
		policies.PreventTimestampsInTheFuture(30),
		rejectCreatingExistingGroups,
	)

	state.Relay.OnConnect = append(state.Relay.OnConnect, khatru.RequestAuth)

	// http routes
	state.Relay.Router().HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "nothing to see here, you must use a nip-29 powered client")
	})

	fmt.Println("running on http://0.0.0.0:" + s.Port)
	if err := http.ListenAndServe(":"+s.Port, state.Relay); err != nil {
		log.Fatal().Err(err).Msg("failed to serve")
	}
}
