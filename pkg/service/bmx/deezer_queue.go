package bmx

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/stegerj/bose-soundtouch/pkg/client"
	"github.com/stegerj/bose-soundtouch/pkg/models"
)

// QueueTrack describes a single playable item, identified by its Deezer
// catalog ID. The device resolves and streams audio from that ID natively.
type QueueTrack struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	CoverURL string `json:"cover_url"`
}

// QueueSnapshot is what the UI receives on every state change.
//   - Playing: current is set, playing=true
//   - Paused:  current is nil, paused=true, upcoming has the parked list
//   - Empty:   everything nil/false/empty
type QueueSnapshot struct {
	Current  *QueueTrack  `json:"current"`
	Upcoming []QueueTrack `json:"upcoming"`
	Playing  bool         `json:"playing"`
	Paused   bool         `json:"paused"`
}

// nowPlayingEvent is the internal representation of a NowPlaying WebSocket
// event, distilled to the fields the track-end state machine needs.
type nowPlayingEvent struct {
	status   models.PlayStatus
	source   string
	location string // ContentItem.Location — the track ID the device is playing
}

type Queue struct {
	mu            sync.Mutex
	deviceIP      string
	tracks        []QueueTrack // tracks[0] = currently playing; shrinks as consumed
	stop          chan struct{} // close to stop playback (parks remaining tracks)
	skip          chan struct{} // send to skip current track
	spkClient     *client.Client
	sourceAccount string
	// npEvents carries NowPlaying events from the single queue-lifetime WebSocket
	// connection into waitForTrackEnd. Buffered so the WS callback never blocks.
	npEvents chan nowPlayingEvent
}

var (
	activeQueues   = map[string]*Queue{}
	activeQueuesMu sync.Mutex

	parkedTracks = map[string][]QueueTrack{}
	parkedMu     sync.Mutex
)

// ── broadcaster ───────────────────────────────────────────────────────────────

var (
	queueBroadcaster   func(deviceIP string, snap QueueSnapshot)
	queueBroadcasterMu sync.RWMutex
)

func RegisterQueueBroadcaster(fn func(deviceIP string, snap QueueSnapshot)) {
	queueBroadcasterMu.Lock()
	queueBroadcaster = fn
	queueBroadcasterMu.Unlock()
}

func notifyQueueChange(deviceIP string) {
	queueBroadcasterMu.RLock()
	fn := queueBroadcaster
	queueBroadcasterMu.RUnlock()
	if fn != nil {
		fn(deviceIP, GetQueueSnapshot(deviceIP))
	}
}

// keyClient is a lightweight HTTP client used only for /key commands.
// Self-contained so deezer_queue.go has no dependency on httpClient in deezer.go.
var keyClient = &http.Client{Timeout: 5 * time.Second}

// sendDeviceKey sends a key press+release pair to the device's /key endpoint.
func sendDeviceKey(deviceIP, keyName string) {
	apiURL := fmt.Sprintf("http://%s:8090/key", deviceIP)
	for _, state := range []string{"press", "release"} {
		body := fmt.Sprintf(`<key state="%s" sender="Gabbo">%s</key>`, state, keyName)
		req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/xml")
		if resp, err := keyClient.Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}
}

// ── public API ────────────────────────────────────────────────────────────────

func ReplaceQueue(deviceIP string, tracks []QueueTrack) *Queue {
	parkedMu.Lock()
	delete(parkedTracks, deviceIP)
	parkedMu.Unlock()

	q := startQueue(deviceIP, tracks)
	notifyQueueChange(deviceIP)
	return q
}

func AppendQueue(deviceIP string, tracks []QueueTrack) {
	activeQueuesMu.Lock()
	q, running := activeQueues[deviceIP]
	activeQueuesMu.Unlock()

	if running {
		q.mu.Lock()
		q.tracks = append(q.tracks, tracks...)
		q.mu.Unlock()
	} else {
		parkedMu.Lock()
		parked := parkedTracks[deviceIP]
		if len(parked) > 0 {
			parkedTracks[deviceIP] = append(parked, tracks...)
			parkedMu.Unlock()
		} else {
			parkedMu.Unlock()
			startQueue(deviceIP, tracks)
		}
	}
	notifyQueueChange(deviceIP)
}

func StopQueue(deviceIP string) {
	activeQueuesMu.Lock()
	q, ok := activeQueues[deviceIP]
	if ok {
		q.mu.Lock()
		remaining := append([]QueueTrack{}, q.tracks...)
		q.mu.Unlock()
		if len(remaining) > 0 {
			parkedMu.Lock()
			parkedTracks[deviceIP] = remaining
			parkedMu.Unlock()
		}
		close(q.stop)
		delete(activeQueues, deviceIP)
	}
	activeQueuesMu.Unlock()

	if ok {
		sendDeviceKey(deviceIP, "PAUSE")
	}
	notifyQueueChange(deviceIP)
}

func PlayQueue(deviceIP string) error {
	activeQueuesMu.Lock()
	_, running := activeQueues[deviceIP]
	activeQueuesMu.Unlock()
	if running {
		return nil
	}

	parkedMu.Lock()
	tracks, ok := parkedTracks[deviceIP]
	delete(parkedTracks, deviceIP)
	parkedMu.Unlock()

	if !ok || len(tracks) == 0 {
		return fmt.Errorf("no parked tracks to resume")
	}
	startQueue(deviceIP, tracks)
	notifyQueueChange(deviceIP)
	return nil
}

func SkipTrack(deviceIP string) {
	activeQueuesMu.Lock()
	q, ok := activeQueues[deviceIP]
	activeQueuesMu.Unlock()
	if ok {
		select {
		case q.skip <- struct{}{}:
		default:
		}
	}
}

func RemoveFromQueue(deviceIP string, index int) error {
	activeQueuesMu.Lock()
	q, running := activeQueues[deviceIP]
	activeQueuesMu.Unlock()
	if !running {
		return fmt.Errorf("no active queue")
	}

	q.mu.Lock()
	upcoming := q.tracks[1:]
	if index < 0 || index >= len(upcoming) {
		q.mu.Unlock()
		return fmt.Errorf("index out of range")
	}
	q.tracks = append(q.tracks[:1+index], q.tracks[2+index:]...)
	q.mu.Unlock()

	notifyQueueChange(deviceIP)
	return nil
}

func ClearUpcoming(deviceIP string) {
	activeQueuesMu.Lock()
	q, ok := activeQueues[deviceIP]
	activeQueuesMu.Unlock()
	if ok {
		q.mu.Lock()
		if len(q.tracks) > 1 {
			q.tracks = q.tracks[:1]
		}
		q.mu.Unlock()
	}
	parkedMu.Lock()
	delete(parkedTracks, deviceIP)
	parkedMu.Unlock()
	notifyQueueChange(deviceIP)
}

func GetQueueSnapshot(deviceIP string) QueueSnapshot {
	activeQueuesMu.Lock()
	q, running := activeQueues[deviceIP]
	activeQueuesMu.Unlock()

	if running {
		q.mu.Lock()
		defer q.mu.Unlock()
		if len(q.tracks) == 0 {
			return QueueSnapshot{Upcoming: []QueueTrack{}}
		}
		cur := q.tracks[0]
		upcoming := make([]QueueTrack, len(q.tracks)-1)
		copy(upcoming, q.tracks[1:])
		return QueueSnapshot{Current: &cur, Upcoming: upcoming, Playing: true}
	}

	parkedMu.Lock()
	parked := parkedTracks[deviceIP]
	parkedMu.Unlock()
	if len(parked) > 0 {
		upcoming := make([]QueueTrack, len(parked))
		copy(upcoming, parked)
		return QueueSnapshot{Upcoming: upcoming, Paused: true}
	}
	return QueueSnapshot{Upcoming: []QueueTrack{}}
}

// ── internal ──────────────────────────────────────────────────────────────────

func startQueue(deviceIP string, tracks []QueueTrack) *Queue {
	q := &Queue{
		deviceIP:      deviceIP,
		tracks:        append([]QueueTrack{}, tracks...),
		stop:          make(chan struct{}),
		skip:          make(chan struct{}, 1),
		spkClient:     client.NewClientFromHost(deviceIP),
		sourceAccount: deezerAccountOrFallback(deviceIP),
		// Buffer 8 events so the WS callback never blocks the device read loop.
		npEvents: make(chan nowPlayingEvent, 8),
	}

	activeQueuesMu.Lock()
	if old, ok := activeQueues[deviceIP]; ok {
		close(old.stop)
	}
	activeQueues[deviceIP] = q
	activeQueuesMu.Unlock()

	go q.run()
	return q
}

func deezerAccountOrFallback(deviceIP string) string {
	if acct := DeezerSourceAccount(deviceIP); acct != "" {
		return acct
	}
	return "12345678"
}

func (q *Queue) run() {
	defer func() {
		activeQueuesMu.Lock()
		if cur, ok := activeQueues[q.deviceIP]; ok && cur == q {
			delete(activeQueues, q.deviceIP)
		}
		activeQueuesMu.Unlock()
		notifyQueueChange(q.deviceIP)
	}()

	// Open ONE WebSocket for the entire queue lifetime.
	// The previous design opened+closed a fresh socket per track; that caused
	// missed STOP events (the speaker doesn't replay events on reconnect) and
	// unnecessary churn on a small-server deployment.
	wsClient := q.spkClient.NewWebSocketClient(nil)
	wsClient.OnNowPlaying(func(event *models.NowPlayingUpdatedEvent) {
		np := event.NowPlaying
		ev := nowPlayingEvent{
			status: np.PlayStatus,
			source: np.Source,
		}
		// ContentItem.Location holds the Deezer track ID the device is actually
		// playing. If the field name differs in your models build, adjust here.
		if ci := np.ContentItem; ci != nil {
			ev.location = ci.Location
		}
		select {
		case q.npEvents <- ev:
		default:
			// Channel full: drop the oldest and enqueue the new one so we
			// always have the freshest status without blocking the WS loop.
			select {
			case <-q.npEvents:
			default:
			}
			select {
			case q.npEvents <- ev:
			default:
			}
		}
	})

	wsOK := wsClient.Connect() == nil
	if !wsOK {
		log.Printf("[deezer-queue] WebSocket unavailable — falling back to polling")
	} else {
		defer wsClient.Disconnect()
	}

	for {
		q.mu.Lock()
		if len(q.tracks) == 0 {
			q.mu.Unlock()
			return
		}
		track := q.tracks[0]
		q.mu.Unlock()

		log.Printf("[deezer-queue] playing (%d in queue): %s — %s",
			q.tracksLen(), track.Title, track.Artist)

		if err := q.playTrack(track); err != nil {
			log.Printf("[deezer-queue] playTrack error: %v", err)
			return
		}

		if !q.waitForTrackEnd(track, wsOK) {
			return
		}

		q.mu.Lock()
		if len(q.tracks) > 0 {
			q.tracks = q.tracks[1:]
		}
		q.mu.Unlock()
		notifyQueueChange(q.deviceIP)
	}
}

func (q *Queue) tracksLen() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.tracks)
}

func (q *Queue) playTrack(track QueueTrack) error {
	return q.spkClient.SelectContentItem(&models.ContentItem{
		Source:        "DEEZER",
		Type:          "track",
		Location:      fmt.Sprintf("%d", track.ID),
		ItemName:      track.Title + " — " + track.Artist,
		SourceAccount: q.sourceAccount,
		IsPresetable:  true,
	})
}

// waitForTrackEnd implements a state-machine that reliably detects real
// end-of-track without being fooled by transient rebuffer STOPs.
//
// State machine:
//
//  1. WAITING_FOR_PLAY: ignore all STOP/STANDBY events until we see at least
//     one PLAYING event for this track (replaces the old fixed 8s cooldown,
//     which was too short for slow networks and too long for short tracks).
//
//  2. PLAYING_CONFIRMED: on STOP/STANDBY, debounce: wait 2.5s then do a
//     synchronous REST check. If the device is still stopped → advance.
//     If it's playing again → rebuffer blip, return to PLAYING_CONFIRMED.
//
// Additional signals:
//   - INVALID_SOURCE: track failed to load → advance immediately.
//   - skip channel: user pressed Next → advance immediately.
//   - 30s with no PLAY seen: track silently failed to start → advance.
//   - 10-min hard deadline: safety net for any unrecognised state.
func (q *Queue) waitForTrackEnd(track QueueTrack, wsOK bool) bool {
	if !wsOK {
		return q.pollForTrackEnd()
	}

	// Drain stale events from the previous track before we start watching.
	for len(q.npEvents) > 0 {
		<-q.npEvents
	}

	const (
		noPlayTimeout  = 30 * time.Second  // track never started playing
		debounceStop   = 2500 * time.Millisecond
		hardTimeout    = 10 * time.Minute
	)

	trackID := fmt.Sprintf("%d", track.ID)
	playConfirmed := false

	hardDeadline := time.NewTimer(hardTimeout)
	defer hardDeadline.Stop()

	noPlayTimer := time.NewTimer(noPlayTimeout)
	defer noPlayTimer.Stop()

	for {
		select {
		case <-q.stop:
			return false

		case <-q.skip:
			log.Printf("[deezer-queue] skip: %s", track.Title)
			return true

		case <-hardDeadline.C:
			log.Printf("[deezer-queue] hard timeout — advancing past: %s", track.Title)
			return true

		case <-noPlayTimer.C:
			if !playConfirmed {
				log.Printf("[deezer-queue] no PLAY in 30s — advancing past: %s", track.Title)
				return true
			}
			// playConfirmed became true after noPlayTimer was created; the
			// case fired anyway (can't be un-selected). Just keep looping.

		case ev := <-q.npEvents:
			// Best-effort: if the device reports which track it's playing and
			// it doesn't match ours, the event belongs to a foreign source
			// (e.g. the user switched input manually). Ignore it.
			if ev.location != "" && ev.location != trackID {
				log.Printf("[deezer-queue] ignoring event for location %s (want %s)",
					ev.location, trackID)
				continue
			}

			if ev.source == "INVALID_SOURCE" {
				log.Printf("[deezer-queue] INVALID_SOURCE — advancing past: %s", track.Title)
				return true
			}

			switch ev.status {
			case models.PlayStatusPlaying:
				if !playConfirmed {
					log.Printf("[deezer-queue] PLAY confirmed: %s", track.Title)
					playConfirmed = true
					// Stop the no-play timer; drain it if it already fired.
					if !noPlayTimer.Stop() {
						select {
						case <-noPlayTimer.C:
						default:
						}
					}
				}

			case models.PlayStatusStopped, models.PlayStatusStandby:
				if !playConfirmed {
					// STOP before PLAY: normal during initial buffering. Ignore.
					continue
				}
				// STOP after PLAY: probably real end-of-track, but debounce
				// to rule out a mid-track rebuffer blip.
				log.Printf("[deezer-queue] STOP seen — debouncing (2.5s): %s", track.Title)
				select {
				case <-time.After(debounceStop):
				case <-q.stop:
					return false
				case <-q.skip:
					return true
				}
				// Synchronous REST check: if the device is truly idle, advance.
				np, err := q.spkClient.GetNowPlaying()
				if err != nil {
					log.Printf("[deezer-queue] debounce REST error: %v — advancing", err)
					return true
				}
				if np.PlayStatus == models.PlayStatusStopped ||
					np.PlayStatus == models.PlayStatusStandby ||
					np.Source == "INVALID_SOURCE" {
					log.Printf("[deezer-queue] STOP confirmed — advancing past: %s", track.Title)
					return true
				}
				// Device is playing again — it was a transient rebuffer STOP.
				log.Printf("[deezer-queue] STOP was transient rebuffer — continuing: %s", track.Title)
			}
		}
	}
}

// pollForTrackEnd is the fallback when WebSocket is unavailable. Polls the
// device REST endpoint every 5 s with the same state machine semantics:
// ignore STOP until PLAY has been seen at least once.
func (q *Queue) pollForTrackEnd() bool {
	const (
		pollInterval  = 5 * time.Second
		noPlayTimeout = 60 * time.Second // generous: no WS = slower feedback
		hardTimeout   = 10 * time.Minute
	)

	playConfirmed := false
	start := time.Now()

	deadline := time.NewTimer(hardTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-q.stop:
			return false
		case <-q.skip:
			return true
		case <-deadline.C:
			log.Printf("[deezer-queue] poll hard timeout — advancing")
			return true
		case <-ticker.C:
			if !playConfirmed && time.Since(start) > noPlayTimeout {
				log.Printf("[deezer-queue] poll: no PLAY in 60s — advancing")
				return true
			}
			np, err := q.spkClient.GetNowPlaying()
			if err != nil {
				log.Printf("[deezer-queue] poll error: %v", err)
				continue
			}
			if np.Source == "INVALID_SOURCE" {
				return true
			}
			switch np.PlayStatus {
			case models.PlayStatusPlaying:
				playConfirmed = true
			case models.PlayStatusStopped, models.PlayStatusStandby:
				if playConfirmed {
					log.Printf("[deezer-queue] poll: STOP confirmed — advancing")
					return true
				}
			}
		}
	}
}
