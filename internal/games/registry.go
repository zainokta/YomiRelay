package games

import (
	"sort"
	"strings"
	"sync"
	"time"

	"yomirelay/internal/steam"
)

// Game is the API-facing state of a detected Ren'Py installation.
type Game struct {
	AppID         string     `json:"appId"`
	Name          string     `json:"name"`
	InstallPath   string     `json:"installPath"`
	Engine        string     `json:"engine"`
	HookInstalled bool       `json:"hookInstalled"`
	Active        bool       `json:"active"`
	LastSeen      *time.Time `json:"lastSeen,omitempty"`
}

type DiscoverFunc func() ([]steam.Installation, error)
type HookStatusFunc func(Game) bool
type ActivityFunc func(string) (lastSeen time.Time, active bool, known bool)

// Registry contains the currently detected games.
type Registry struct {
	mu       sync.RWMutex
	discover DiscoverFunc
	hooks    HookStatusFunc
	activity ActivityFunc
	games    map[string]Game
}

func NewRegistry(discover DiscoverFunc, hooks HookStatusFunc, activity ActivityFunc) *Registry {
	return &Registry{discover: discover, hooks: hooks, activity: activity, games: make(map[string]Game)}
}

// Refresh replaces the registry only after discovery succeeds.
func (r *Registry) Refresh() error {
	installations, err := r.discover()
	if err != nil {
		return err
	}
	next := make(map[string]Game, len(installations))
	for _, installation := range installations {
		if !IsRenPy(installation.InstallPath) {
			continue
		}
		game := Game{AppID: installation.AppID, Name: installation.Name, InstallPath: installation.InstallPath, Engine: "renpy"}
		if r.hooks != nil {
			game.HookInstalled = r.hooks(game)
		}
		next[game.AppID] = game
	}
	r.mu.Lock()
	r.games = next
	r.mu.Unlock()
	return nil
}

func (r *Registry) Get(appID string) (Game, bool) {
	r.mu.RLock()
	game, ok := r.games[appID]
	r.mu.RUnlock()
	if !ok {
		return Game{}, false
	}
	return r.withActivity(game), true
}

func (r *Registry) List() []Game {
	r.mu.RLock()
	games := make([]Game, 0, len(r.games))
	for _, game := range r.games {
		games = append(games, game)
	}
	r.mu.RUnlock()
	for i := range games {
		games[i] = r.withActivity(games[i])
	}
	sort.Slice(games, func(i, j int) bool {
		left, right := strings.ToLower(games[i].Name), strings.ToLower(games[j].Name)
		if left != right {
			return left < right
		}
		return games[i].AppID < games[j].AppID
	})
	return games
}

func (r *Registry) withActivity(game Game) Game {
	if r.activity == nil {
		return game
	}
	lastSeen, active, known := r.activity(game.AppID)
	game.Active = active
	game.LastSeen = nil
	if known && !lastSeen.IsZero() {
		seen := lastSeen
		game.LastSeen = &seen
	}
	return game
}
