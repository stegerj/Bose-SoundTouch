package bmx

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gesellix/bose-soundtouch/pkg/client"
	"github.com/gesellix/bose-soundtouch/pkg/models"
)

// QueueTrack describes a single playable item in a queue, identified by its
// Deezer catalog ID. The device's own Deezer integration resolves and
// streams the audio from that ID — the same mechanism the classic single
// track/album play path (HandlePlayDeezer) already uses successfully.
type QueueTrack struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	CoverURL string `json:"cover_url"`
}

// QueueSnapshot is what the UI polls for: the currently-playing track
// (nil when idle) and the remaining upcoming tracks.
type QueueSnapshot struct {
	Current  *QueueTrack  `json:"current"`
	Upcoming []QueueTrack `json:"upcoming"`
	Playing  bool         `json:"playing"`
}

type Queue struct {
	mu            sync.Mutex
	deviceIP      string
	tracks        []QueueTrack // shrinks as tracks are consumed
	stop          chan struct{}
	spkClient     *client.Client
	sourceAccount string
}

var (
	activeQueues   = map[string]*Queue{}
	activeQueuesMu sync.Mutex
)

// ReplaceQueue stops any running queue on deviceIP, replaces it with the
// given tracks and starts playback immediately. Used for ▶ actions.
func ReplaceQueue(deviceIP string, tracks []QueueTrack) *Queue {
	return startQueue(deviceIP, tracks)
}

// AppendQueue adds tracks to the end of the running queue. If nothing is
// running it starts playback immediately. Used for + actions.
func AppendQueue(deviceIP string, tracks []QueueTrack) {
	activeQueuesMu.Lock()
	q, running := activeQueues[deviceIP]
	activeQueuesMu.Unlock()

	if running {
		q.mu.Lock()
		q.tracks = append(q.tracks, tracks...)
		q.mu.Unlock()
		return
	}
	startQueue(deviceIP, tracks)
}

// StopQueue cancels the active queue for deviceIP.
func StopQueue(deviceIP string) {
	activeQueuesMu.Lock()
	defer activeQueuesMu.Unlock()
	if q, ok := activeQueues[deviceIP]; ok {
		close(q.stop)
		delete(activeQueues, deviceIP)
	}
}

// RemoveFromQueue removes the upcoming track at index (0 = first upcoming,
// not the currently-playing one). Returns an error if out of range.
func RemoveFromQueue(deviceIP string, index int) error {
	activeQueuesMu.Lock()
	q, running := activeQueues[deviceIP]
	activeQueuesMu.Unlock()
	if !running {
		return fmt.Errorf("no active queue")
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	// tracks[0] is the currently-playing track; upcoming starts at [1].
	upcoming := q.tracks[1:]
	if index < 0 || index >= len(upcoming) {
		return fmt.Errorf("index out of range")
	}
	q.tracks = append(q.tracks[:1+index], q.tracks[2+index:]...)
	return nil
}

// GetQueueSnapshot returns the current queue state for the UI.
func GetQueueSnapshot(deviceIP string) QueueSnapshot {
	activeQueuesMu.Lock()
	q, running := activeQueues[deviceIP]
	activeQueuesMu.Unlock()

	if !running {
		return QueueSnapshot{Upcoming: []QueueTrack{}}
	}

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

// ─── internal ────────────────────────────────────────────────────────────────

func startQueue(deviceIP string, tracks []QueueTrack) *Queue {
	q := &Queue{
		deviceIP:      deviceIP,
		tracks:        append([]QueueTrack{}, tracks...),
		stop:          make(chan struct{}),
		spkClient:     client.NewClientFromHost(deviceIP),
		sourceAccount: deezerAccountOrFallback(deviceIP),
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
	}()

	for {
		// Peek at the first track without removing it yet — the snapshot
		// needs it to show "currently playing".
		q.mu.Lock()
		if len(q.tracks) == 0 {
			q.mu.Unlock()
			return
		}
		track := q.tracks[0]
		q.mu.Unlock()

		log.Printf("[deezer-queue] Playing (%d remaining): %s — %s", q.tracksLen(), track.Title, track.Artist)
		if err := q.playTrack(track); err != nil {
			log.Printf("[deezer-queue] playTrack error: %v", err)
			return
		}

		// Block until the device signals the track ended, or we're stopped.
		if !q.waitForTrackEnd() {
			return // stop requested
		}

		// Track finished: drop it from the front.
		q.mu.Lock()
		if len(q.tracks) > 0 {
			q.tracks = q.tracks[1:]
		}
		q.mu.Unlock()
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

// waitForTrackEnd blocks until the device signals the track has finished,
// then returns true. Returns false only if StopQueue was called.
//
// We only trigger on STOPPED / STANDBY, never on PLAYING.
//
// Why not PLAYING: the device sends routine NowPlaying updates with
// PlayStatusPlaying throughout normal playback (position ticks, re-buffer
// events, etc.). Treating those as "track ended" would advance the queue
// after a few seconds every time — exactly the bug this fixes. STOPPED fires
// reliably when a native Deezer track actually ends; we just need the
// cooldown to outlast the brief STOP→PLAY transition that happens while the
// device is first buffering the track we just sent it.
//
// Cooldown is 8 s: native Deezer tracks can take 5–6 s to buffer on a slow
// network, during which the device may send STOPPED transiently. Once past
// the cooldown the first genuine STOPPED/STANDBY is the real end-of-track.
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
		if np.PlayStatus == models.PlayStatusStopped || np.PlayStatus == models.PlayStatusStandby {
			closeDone()
		}
	})

	if err := wsClient.Connect(); err != nil {
		log.Printf("[deezer-queue] WebSocket connect error: %v — falling back to poll", err)
		// Without WebSocket, poll the device REST status every 5 s.
		return q.pollForTrackEnd()
	}
	defer wsClient.Disconnect()

	select {
	case <-done:
		// Small grace period so the device finishes its own state transition
		// before we immediately fire the next SelectContentItem.
		time.Sleep(500 * time.Millisecond)
		return true
	case <-q.stop:
		return false
	}
}

// pollForTrackEnd is the fallback used when the WebSocket connection fails.
// It polls the device's REST nowPlaying endpoint every 5 seconds and returns
// true once the device reports STOPPED or STANDBY past the startup cooldown.
func (q *Queue) pollForTrackEnd() bool {
	startTime := time.Now()
	const cooldown = 8 * time.Second

	for {
		select {
		case <-q.stop:
			return false
		case <-time.After(5 * time.Second):
		}

		if time.Since(startTime) < cooldown {
			continue
		}

		np, err := q.spkClient.GetNowPlaying()
		if err != nil {
			log.Printf("[deezer-queue] poll GetNowPlaying error: %v", err)
			continue
		}
		if np.PlayStatus == models.PlayStatusStopped || np.PlayStatus == models.PlayStatusStandby {
			return true
		}
	}
}
