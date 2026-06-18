package bmx

import (
	"encoding/xml" // Assicurato
	"fmt"
	"log"
	"net/http" // Assicurato
	"strconv"
	"sync"
	"time"

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
	StreamURL string `json:"stream_url"`
	CoverURL  string `json:"cover_url"`
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

		// Controllo basato sull'ID nativo anziché sulla presenza della preview URL
		if track.ID == 0 && track.StreamURL == "" {
			log.Printf("[deezer-queue] Skipping track %d/%d (%s): no ID or preview URL available", q.pos+1, len(q.tracks), track.Title)
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

	// === SOLUZIONE ORDINATA: Ricaviamo l'ID nativo in formato stringa ===
	locationID := track.StreamURL
	if track.ID != 0 {
		locationID = strconv.FormatInt(track.ID, 10)
	}
	// ====================================================================

	log.Printf("[deezer-queue] Inoltro traccia nativa a Bose -> ID: %s, Account: %s", locationID, sourceAccount)

	// 2. Inviamo i parametri convertendoli nei valori nativi richiesti dal firmware Bose per Deezer
	return q.spkClient.SelectContentItem(&models.ContentItem{
		Source:        "DEEZER",                    // Sorgente nativa ufficiale
		Type:          "track",                     // Indica che è un brano singolo
		Location:      locationID,                  // Stringa numerica pulita dell'ID della traccia (es. "275050252")
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

		// Cooldown iniziale di 4 secondi per evitare cambi traccia istantanei dovuti allo switch
		if time.Since(startTime) < 4*time.Second {
			return
		}

		// Rileviamo se la cassa ha fermato la traccia precedente, è andata in standby
		// o ha momentaneamente perso la sorgente (INVALID_SOURCE) a fine traccia
		isStopped := np.PlayStatus == models.PlayStatusStopped ||
			np.PlayStatus == models.PlayStatusStandby ||
			np.Source == "INVALID_SOURCE"

		// Rileviamo anche se la cassa è ancora in riproduzione ma ha già resettato/svuotato
		// la Location della traccia corrente (segno che ha terminato il file di Deezer)
		q.mu.Lock()
		currentTrack := q.tracks[q.pos]
		q.mu.Unlock()
		idStr := strconv.FormatInt(currentTrack.ID, 10)

		isTrackFinished := np.Source == "DEEZER" && np.ContentItem.Location != idStr

		if isStopped || isTrackFinished {
			log.Printf("[deezer-queue] Cambio traccia intercettato (Stato: %s, Sorgente: %s). Preparazione transizione...", np.PlayStatus, np.Source)

			// PICCOLO TRUCCO DI SINCRONIZZAZIONE (Anti INVALID_SOURCE)
			// Aspettiamo un istante per far scaricare l'ultimo micro-buffer audio della cassa,
			// dopodiché chiudiamo il canale per forzare l'invio immediato del prossimo brano.
			go func() {
				time.Sleep(200 * time.Millisecond)
				select {
				case <-done:
				default:
					close(done)
				}
			}()
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
// ============================================================================

var (
	visibleQueueTracks = map[string][]QueueTrack{}
	visibleQueueOwner  = map[string]*Queue{}
	visibleQueueMu     sync.Mutex
)

type VisibleQueueSnapshot struct {
	Tracks  []QueueTrack `json:"tracks"`
	Playing bool         `json:"playing"`
	Pos     int          `json:"pos"`
}

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

func GetVisibleQueueSnapshot(deviceIP string) VisibleQueueSnapshot {
	visibleQueueMu.Lock()
	defer visibleQueueMu.Unlock()

	tracks := visibleQueueTracks[deviceIP]
	if tracks == nil {
		tracks = []QueueTrack{}
	}

	owner := visibleQueueOwner[deviceIP]
	if owner == nil {
		return VisibleQueueSnapshot{Tracks: tracks, Playing: false, Pos: 0}
	}

	activeQueuesMu.Lock()
	stillActive := activeQueues[deviceIP] == owner
	activeQueuesMu.Unlock()

	if !stillActive {
		return VisibleQueueSnapshot{Tracks: tracks, Playing: false, Pos: 0}
	}

	owner.mu.Lock()
	pos := owner.pos
	owner.mu.Unlock()

	return VisibleQueueSnapshot{Tracks: tracks, Playing: true, Pos: pos}
}

func RemoveFromVisibleQueue(deviceIP string, index int) error {
	visibleQueueMu.Lock()
	defer visibleQueueMu.Unlock()

	tracks := visibleQueueTracks[deviceIP]
	if index < 0 || index >= len(tracks) {
		return fmt.Errorf("index out of bounds")
	}

	visibleQueueTracks[deviceIP] = append(tracks[:index], tracks[index+1:]...)

	owner := visibleQueueOwner[deviceIP]
	if owner == nil {
		return nil
	}

	activeQueuesMu.Lock()
	stillActive := activeQueues[deviceIP] == owner
	activeQueuesMu.Unlock()

	if stillActive {
		owner.mu.Lock()
		if index < owner.pos {
			owner.pos--
		}
		owner.tracks = append(owner.tracks[:index], owner.tracks[index+1:]...)
		owner.mu.Unlock()
	}

	return nil
}

func ClearVisibleQueue(deviceIP string) {
	visibleQueueMu.Lock()
	visibleQueueTracks[deviceIP] = []QueueTrack{}
	owner := visibleQueueOwner[deviceIP]
	visibleQueueMu.Unlock()

	if owner == nil {
		return
	}

	activeQueuesMu.Lock()
	stillActive := activeQueues[deviceIP] == owner
	activeQueuesMu.Unlock()

	if stillActive {
		close(owner.stop)
	}
}
