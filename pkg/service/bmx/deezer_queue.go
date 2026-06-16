package bmx

import (
	"fmt"
	"log"
	"sync"
	"time"
	"encoding/xml" // <-- ASSICURATI CHE SIA PRESENTE
    "net/http"     // <-- ASSICURATI CHE SIA PRESENTE

	"github.com/gesellix/bose-soundtouch/pkg/client"
	"github.com/gesellix/bose-soundtouch/pkg/models"
)

// QueueTrack describes a single playable item in a queue. StreamURL must be
// a directly playable audio URL (Deezer's "preview" clip URL — the only
// audio obtainable from the public, unauthenticated Deezer API), NOT a
// Deezer catalog ID. A track with an empty StreamURL is skipped during
// playback rather than sent to the device.
type QueueTrack struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	StreamURL string `json:"stream_url"` // Obbligatorio in snake_case per allinearsi a api.js
	CoverURL  string `json:"cover_url"`  // Obbligatorio in snake_case per allinearsi a api.js
}

type Queue struct {
	mu        sync.Mutex
	deviceIP  string
	tracks    []QueueTrack
	pos       int
	stop      chan struct{}
	spkClient *client.Client
}

var (
	activeQueues   = map[string]*Queue{}
	activeQueuesMu sync.Mutex
)

// StartQueue replaces and starts a "hidden" one-off queue for a device (used
// for "continue playing the rest of this album/tracklist" actions). It does
// not touch the persistent visible queue below.
func StartQueue(deviceIP string, tracks []QueueTrack) {
	StartQueueTracking(deviceIP, tracks)
}

// StartQueueTracking behaves exactly like StartQueue but returns the created
// *Queue, letting callers (PlayVisibleQueue) keep a handle to check later
// whether it's still the thing actively driving playback.
func StartQueueTracking(deviceIP string, tracks []QueueTrack) *Queue {
	q := &Queue{
		deviceIP:  deviceIP,
		tracks:    tracks,
		stop:      make(chan struct{}),
		spkClient: client.NewClientFromHost(deviceIP),
	}

	activeQueuesMu.Lock()
	if old, ok := activeQueues[deviceIP]; ok {
		close(old.stop) // Segnala alla vecchia coda di fermarsi
	}
	activeQueues[deviceIP] = q
	activeQueuesMu.Unlock()

	go q.run()
	return q
}

func StopQueue(deviceIP string) {
	activeQueuesMu.Lock()
	defer activeQueuesMu.Unlock()
	if q, ok := activeQueues[deviceIP]; ok {
		close(q.stop)
		delete(activeQueues, deviceIP)
	}
}

func (q *Queue) run() {
	defer func() {
		activeQueuesMu.Lock()
		if current, ok := activeQueues[q.deviceIP]; ok && current == q {
			delete(activeQueues, q.deviceIP)
		}
		activeQueuesMu.Unlock()
	}()

	for {
		q.mu.Lock()
		if q.pos >= len(q.tracks) {
			q.mu.Unlock()
			return
		}
		track := q.tracks[q.pos]
		q.mu.Unlock()

		if track.StreamURL == "" {
			log.Printf("[deezer-queue] Skipping track %d/%d (%s): no preview URL available", q.pos+1, len(q.tracks), track.Title)
			q.mu.Lock()
			q.pos++
			q.mu.Unlock()
			continue
		}

		log.Printf("[deezer-queue] Playing track %d/%d: %s", q.pos+1, len(q.tracks), track.Title)
		if err := q.playTrack(track); err != nil {
			log.Printf("[deezer-queue] playTrack error: %v", err)
			return
		}

		if !q.waitForTrackEnd() {
			return
		}

		q.mu.Lock()
		q.pos++
		q.mu.Unlock()
	}
}

func (q *Queue) playTrack(track QueueTrack) error {
	// 1. Recuperiamo dinamicamente l'account Deezer reale registrato su questo diffusore.
	// Nota: Poiché siamo all'interno del pacchetto bmx e non abbiamo l'istanza 'app' del server web,
	// interroghiamo direttamente l'endpoint locale della cassa sulla porta 8090 per estrarre l'account.
	sourceAccount := ""
	sourcesURL := fmt.Sprintf("http://%s:8090/sources", q.deviceIP)

	if resp, err := http.Get(sourcesURL); err == nil {
		var result struct {
			SourceItems []struct {
				Source        string `xml:"source,attr"`
				SourceAccount string `xml:"sourceAccount,attr"`
				Status        string `xml:"status,attr"`
			} `xml:"sourceItem"`
		}

		// Usiamo il decodificatore XML nativo di Go per interpretare i dati della cassa
		if err := xml.NewDecoder(resp.Body).Decode(&result); err == nil {
			for _, item := range result.SourceItems {
				if item.Source == "DEEZER" && item.Status == "READY" {
					sourceAccount = item.SourceAccount
					break
				}
			}
			if sourceAccount == "" {
				for _, item := range result.SourceItems {
					if item.Source == "DEEZER" {
						sourceAccount = item.SourceAccount
						break
					}
				}
			}
		}
		_ = resp.Body.Close()
	}

	// Fallback di sicurezza se la cassa non restituisce un account valido (ID fittizio post-cloud)
	if sourceAccount == "" {
		sourceAccount = "12345678"
	}

	log.Printf("[deezer-queue] Inoltro traccia nativa a Bose -> ID: %s, Account: %s", track.StreamURL, sourceAccount)

	// 2. Modifichiamo i parametri convertendoli nei valori nativi richiesti dal firmware Bose per Deezer
	return q.spkClient.SelectContentItem(&models.ContentItem{
		Source:        "DEEZER",                    // Sorgente nativa ufficiale
		Type:          "track",                     // Indica che è un brano singolo
		Location:      track.StreamURL,             // Stringa numerica pulita dell'ID della traccia (es. "275050252")
		ItemName:      track.Title + " — " + track.Artist,
		SourceAccount: sourceAccount,               // Identificativo dell'abbonamento Premium sulla cassa
		IsPresetable:  true,
	})
}

func (q *Queue) waitForTrackEnd() bool {
	done := make(chan struct{})
	startTime := time.Now()

	wsClient := q.spkClient.NewWebSocketClient(nil)
	wsClient.OnNowPlaying(func(event *models.NowPlayingUpdatedEvent) {
		np := event.NowPlaying

		// Cooldown per evitare lo STOP_STATE istantaneo del cambio traccia
		if time.Since(startTime) < 3*time.Second {
			return
		}

		if np.PlayStatus == models.PlayStatusStopped || np.PlayStatus == models.PlayStatusStandby {
			select {
			case <-done:
			default:
				close(done)
			}
		}
	})

	if err := wsClient.Connect(); err != nil {
		log.Printf("[deezer-queue] WebSocket connect error: %v", err)
		select {
		case <-time.After(5 * time.Second):
			return true
		case <-q.stop:
			return false
		}
	}
	defer wsClient.Disconnect()

	select {
	case <-done:
		return true
	case <-q.stop:
		return false
	}
}

// ============================================================================
// VISIBLE QUEUE — user-curated, persists across stop/play, shown in the UI.
//
// Unlike the hidden queue above (StartQueue/StopQueue, used for "continue
// playing the rest of this album/tracklist" actions, which is replaced
// wholesale every time it's triggered), the visible queue is built up
// incrementally via AddToVisibleQueue and only starts driving playback when
// PlayVisibleQueue is called. Its track list survives being interrupted by a
// hidden quick-play elsewhere, so pressing Play again resumes the same
// queue from the top.
// ============================================================================

var (
	visibleQueueTracks = map[string][]QueueTrack{}
	visibleQueueOwner  = map[string]*Queue{}
	visibleQueueMu     sync.Mutex
)

// VisibleQueueSnapshot describes the current state of a device's visible
// queue, returned to the UI for display.
type VisibleQueueSnapshot struct {
	Tracks  []QueueTrack `json:"tracks"`
	Playing bool         `json:"playing"`
	Pos     int          `json:"pos"`
}

// AddToVisibleQueue appends tracks to the device's visible queue buffer. If
// the visible queue is the one currently driving playback, the tracks are
// also appended to the live, running queue so they play seamlessly once
// reached — no restart needed.
func AddToVisibleQueue(deviceIP string, tracks []QueueTrack) {
	visibleQueueMu.Lock()
	visibleQueueTracks[deviceIP] = append(visibleQueueTracks[deviceIP], tracks...)
	owner := visibleQueueOwner[deviceIP]
	visibleQueueMu.Unlock()

	if owner == nil {
		return
	}

	activeQueuesMu.Lock()
	stillActive := activeQueues[deviceIP] == owner
	activeQueuesMu.Unlock()

	if stillActive {
		owner.mu.Lock()
		owner.tracks = append(owner.tracks, tracks...)
		owner.mu.Unlock()
	}
}

// PlayVisibleQueue starts (or restarts, from the top) playback of the
// device's visible queue buffer.
func PlayVisibleQueue(deviceIP string) error {
	visibleQueueMu.Lock()
	tracks := append([]QueueTrack{}, visibleQueueTracks[deviceIP]...)
	visibleQueueMu.Unlock()

	if len(tracks) == 0 {
		return fmt.Errorf("visible queue is empty")
	}

	q := StartQueueTracking(deviceIP, tracks)

	visibleQueueMu.Lock()
	visibleQueueOwner[deviceIP] = q
	visibleQueueMu.Unlock()

	return nil
}

// RemoveFromVisibleQueue removes the track at index from the staged buffer.
// It only affects the buffer, not an already-running playback session — the
// UI is expected to disable removal while the queue is playing.
func RemoveFromVisibleQueue(deviceIP string, index int) error {
	visibleQueueMu.Lock()
	defer visibleQueueMu.Unlock()

	tracks := visibleQueueTracks[deviceIP]
	if index < 0 || index >= len(tracks) {
		return fmt.Errorf("index out of range")
	}
	updated := make([]QueueTrack, 0, len(tracks)-1)
	updated = append(updated, tracks[:index]...)
	updated = append(updated, tracks[index+1:]...)
	visibleQueueTracks[deviceIP] = updated
	return nil
}

// ClearVisibleQueue empties the device's visible queue buffer.
func ClearVisibleQueue(deviceIP string) {
	visibleQueueMu.Lock()
	delete(visibleQueueTracks, deviceIP)
	visibleQueueMu.Unlock()
}

// GetVisibleQueueSnapshot returns the current visible queue buffer plus
// whether it's the thing actively driving playback right now.
func GetVisibleQueueSnapshot(deviceIP string) VisibleQueueSnapshot {
	visibleQueueMu.Lock()
	tracks := append([]QueueTrack{}, visibleQueueTracks[deviceIP]...)
	owner := visibleQueueOwner[deviceIP]
	visibleQueueMu.Unlock()

	snapshot := VisibleQueueSnapshot{Tracks: tracks}
	if owner == nil {
		return snapshot
	}

	activeQueuesMu.Lock()
	stillActive := activeQueues[deviceIP] == owner
	activeQueuesMu.Unlock()
	if !stillActive {
		return snapshot
	}

	owner.mu.Lock()
	snapshot.Playing = true
	snapshot.Pos = owner.pos
	owner.mu.Unlock()
	return snapshot
}
