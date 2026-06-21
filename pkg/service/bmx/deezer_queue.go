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

type Queue struct {
	mu            sync.Mutex
	deviceIP      string
	tracks        []QueueTrack  // tracks[0] = currently playing; shrinks as consumed
	stop          chan struct{} // close to stop playback (parks remaining tracks)
	skip          chan struct{} // send to skip current track
	spkClient     *client.Client
	sourceAccount string
}

// httpClient is shared across all Deezer API calls with a sensible timeout.
var httpClient = &http.Client{Timeout: 10 * time.Second}

var (
	activeQueues   = map[string]*Queue{}
	activeQueuesMu sync.Mutex

	// parkedTracks holds the remaining track list after StopQueue.
	// PlayQueue drains it and restarts the goroutine.
	parkedTracks = map[string][]QueueTrack{}
	parkedMu     sync.Mutex
)

// ── broadcaster registration ─────────────────────────────────────────────────

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

// sendDeviceKey sends a key press+release pair to the device's /key REST
// endpoint. The queue uses this for stop/pause commands that must reach the
// hardware, not just the Go-side queue state.
// httpClient is the package-level client defined in deezer.go.
func sendDeviceKey(deviceIP, keyName string) {
	apiURL := fmt.Sprintf("http://%s:8090/key", deviceIP)
	for _, state := range []string{"press", "release"} {
		body := fmt.Sprintf(`<key state="%s" sender="Gabbo">%s</key>`, state, keyName)
		req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/xml")
		if resp, err := httpClient.Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}
}

// ── public API ───────────────────────────────────────────────────────────────

// ReplaceQueue stops any running queue, discards any parked tracks, and
// immediately starts playing the supplied list. Used for ▶ on a result row.
func ReplaceQueue(deviceIP string, tracks []QueueTrack) *Queue {
	parkedMu.Lock()
	delete(parkedTracks, deviceIP)
	parkedMu.Unlock()

	activeQueuesMu.Lock()
	q := _startQueueLocked(deviceIP, tracks)
	activeQueuesMu.Unlock()

	// Notify queue state update asynchronously to avoid blocking execution
	go notifyQueueChange(deviceIP)
	return q
}

// AppendQueue adds tracks to the end of the queue.
//   - If playing: appended live; playback continues without interruption.
//   - If paused:  appended to the parked list.
//   - If empty:   starts playback immediately.
func AppendQueue(deviceIP string, tracks []QueueTrack) {
	activeQueuesMu.Lock()
	q, running := activeQueues[deviceIP]

	if running {
		q.mu.Lock()
		q.tracks = append(q.tracks, tracks...)
		q.mu.Unlock()
		activeQueuesMu.Unlock()
	} else {
		// Acquire parked lock inside activeQueues lock for perfect state atomic evaluation
		parkedMu.Lock()
		parked := parkedTracks[deviceIP]
		if len(parked) > 0 {
			parkedTracks[deviceIP] = append(parked, tracks...)
			parkedMu.Unlock()
			activeQueuesMu.Unlock()
		} else {
			parkedMu.Unlock()
			_startQueueLocked(deviceIP, tracks)
			activeQueuesMu.Unlock()
		}
	}
	go notifyQueueChange(deviceIP)
}

// StopQueue stops playback and parks the remaining track list so PlayQueue
// can resume it later. The currently-playing track is included in the parked
// list so the user can replay it if they want.
func StopQueue(deviceIP string) {
	activeQueuesMu.Lock()
	q, ok := activeQueues[deviceIP]
	if ok {
		// Snapshot BEFORE signalling stop — the goroutine's defer will call
		// notifyQueueChange, which must already see the parked state.
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
		// Tell the device to actually stop playing. PAUSE rather than STOP
		// avoids triggering a standby/power-off on some firmware versions.
		sendDeviceKey(deviceIP, "PAUSE")
	}

	go notifyQueueChange(deviceIP)
}

// PlayQueue resumes playback from a parked (previously stopped) queue.
// Returns an error if there is nothing parked.
func PlayQueue(deviceIP string) error {
	activeQueuesMu.Lock()
	defer activeQueuesMu.Unlock()

	_, running := activeQueues[deviceIP]
	if running {
		return nil // already playing, nothing to do
	}

	parkedMu.Lock()
	tracks, ok := parkedTracks[deviceIP]
	delete(parkedTracks, deviceIP)
	parkedMu.Unlock()

	if !ok || len(tracks) == 0 {
		return fmt.Errorf("no parked tracks to resume")
	}

	_startQueueLocked(deviceIP, tracks)
	go notifyQueueChange(deviceIP)
	return nil
}

// SkipTrack advances to the next track immediately without stopping the queue.
func SkipTrack(deviceIP string) {
	activeQueuesMu.Lock()
	q, ok := activeQueues[deviceIP]
	activeQueuesMu.Unlock()

	if ok {
		select {
		case q.skip <- struct{}{}:
		default: // skip already pending
		}
	}
}

// RemoveFromQueue removes upcoming[index] (0 = first after current).
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

	go notifyQueueChange(deviceIP)
	return nil
}

// ClearUpcoming clears all tracks after the currently-playing one.
// If paused, clears the entire parked list.
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

	go notifyQueueChange(deviceIP)
}

// GetQueueSnapshot returns the current state for the UI.
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
	activeQueuesMu.Lock()
	defer activeQueuesMu.Unlock()
	return _startQueueLocked(deviceIP, tracks)
}

// _startQueueLocked expects activeQueuesMu to be held. It handles the queue initialization
// and ensures old queues for this device exit gracefully.
func _startQueueLocked(deviceIP string, tracks []QueueTrack) *Queue {
	q := &Queue{
		deviceIP:      deviceIP,
		tracks:        append([]QueueTrack{}, tracks...),
		stop:          make(chan struct{}),
		skip:          make(chan struct{}, 1), // buffered so sender never blocks
		spkClient:     client.NewClientFromHost(deviceIP),
		sourceAccount: deezerAccountOrFallback(deviceIP),
	}

	if old, ok := activeQueues[deviceIP]; ok {
		close(old.stop)
	}
	activeQueues[deviceIP] = q

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
	// cleanup runs on every exit path. notifyQueueChange fires AFTER cleanup
	// so the broadcast reflects the post-exit state (parked or empty).
	defer func() {
		activeQueuesMu.Lock()
		if cur, ok := activeQueues[q.deviceIP]; ok && cur == q {
			delete(activeQueues, q.deviceIP)
		}
		activeQueuesMu.Unlock()
		notifyQueueChange(q.deviceIP)
	}()

	for {
		q.mu.Lock()
		if len(q.tracks) == 0 {
			q.mu.Unlock()
			return
		}
		track := q.tracks[0]
		q.mu.Unlock()

		log.Printf("[deezer-queue] Playing (%d remaining): %s — %s",
			q.tracksLen(), track.Title, track.Artist)

		if err := q.playTrack(track); err != nil {
			log.Printf("[deezer-queue] playTrack error: %v", err)
			return
		}

		if !q.waitForTrackEnd() {
			return // stop or skip-to-nothing
		}

		// Drop the finished track and broadcast so the UI updates immediately.
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

// waitForTrackEnd blocks until the track ends, is skipped, or the queue is
// stopped. Returns false only when StopQueue was called (caller should exit).
//
// Three conditions advance to the next track:
//  1. STOPPED / STANDBY       — clean end of track.
//  2. source="INVALID_SOURCE" — device has no Deezer account / track failed
//     to load. Without this we hang forever (observed post-Bose-cloud-shutdown).
//  3. 10-minute hard timeout  — safety net for any state we don't recognise.
//
// We do NOT trigger on PLAYING: the device sends heartbeat NowPlaying updates
// with PlayStatusPlaying throughout normal playback, so treating that as "done"
// would cut every track short.
//
// Cooldown 8 s: buffering a Deezer track can produce a brief transient STOPPED
// before the device settles into PLAYING; we ignore all events for the first 8 s.
func (q *Queue) waitForTrackEnd() bool {
	done := make(chan struct{})
	var doneOnce sync.Once
	closeDone := func() { doneOnce.Do(func() { close(done) }) }

	startTime := time.Now()
	const cooldown = 8 * time.Second

	wsClient := q.spkClient.NewWebSocketClient(nil)
	wsClient.OnNowPlaying(func(event *models.NowPlayingUpdatedEvent) {
		if time.Since(startTime) < cooldown {
			return
		}
		np := event.NowPlaying
		if np.PlayStatus == models.PlayStatusStopped ||
			np.PlayStatus == models.PlayStatusStandby ||
			np.Source == "INVALID_SOURCE" {
			closeDone()
		}
	})

	if err := wsClient.Connect(); err != nil {
		log.Printf("[deezer-queue] WebSocket error: %v — falling back to poll", err)
		return q.pollForTrackEnd()
	}
	defer wsClient.Disconnect()

	select {
	case <-done:
		time.Sleep(500 * time.Millisecond)
		return true
	case <-q.skip:
		log.Printf("[deezer-queue] skip requested")
		return true
	case <-q.stop:
		return false
	case <-time.After(10 * time.Minute):
		log.Printf("[deezer-queue] track timeout — advancing")
		return true
	}
}

// pollForTrackEnd is the WebSocket-unavailable fallback.
func (q *Queue) pollForTrackEnd() bool {
	startTime := time.Now()
	const cooldown = 8 * time.Second
	deadline := time.NewTimer(10 * time.Minute)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-q.stop:
			return false
		case <-q.skip:
			return true
		case <-deadline.C:
			log.Printf("[deezer-queue] poll timeout — advancing")
			return true
		case <-ticker.C:
			if time.Since(startTime) < cooldown {
				continue
			}
			np, err := q.spkClient.GetNowPlaying()
			if err != nil {
				log.Printf("[deezer-queue] poll error: %v", err)
				continue
			}
			if np.PlayStatus == models.PlayStatusStopped ||
				np.PlayStatus == models.PlayStatusStandby ||
				np.Source == "INVALID_SOURCE" {
				return true
			}
		}
	}
}
