package soundtouchweb

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	bmxpkg "github.com/gesellix/bose-soundtouch/pkg/service/bmx"
	"github.com/gesellix/bose-soundtouch/pkg/service/soundtouchweb/webtypes"
	"github.com/go-chi/chi/v5"
)

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// sendSuccessJSON handles standard envelope formatting and content-type headers.
func sendSuccessJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: data}); err != nil {
		log.Printf("[deezer-handler] failed to encode response: %v", err)
	}
}

// ============================================================================
// API HANDLERS
// ============================================================================

// HandleDeezerSearch handles Deezer search requests.
func (app *WebApp) HandleDeezerSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	searchType := chi.URLParam(r, "type")
	if searchType == "" {
		searchType = r.URL.Query().Get("type")
	}

	if query == "" {
		app.sendError(w, "query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	rawItems, err := bmxpkg.DeezerSearch(query, searchType)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if rawItems == nil {
		rawItems = []map[string]interface{}{}
	}

	sendSuccessJSON(w, rawItems)
}

// HandleDeezerQueueReplace replaces the current queue with the supplied
// tracklist and starts playing immediately. This is the ▶ play action.
func (app *WebApp) HandleDeezerQueueReplace(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}
	defer r.Body.Close()

	var req struct {
		Tracks []bmxpkg.QueueTrack `json:"tracks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Tracks) == 0 {
		app.sendError(w, "tracks must not be empty", http.StatusBadRequest)
		return
	}

	bmxpkg.ReplaceQueue(device.DeviceInfo.IPAddress, req.Tracks)
	snap := bmxpkg.GetQueueSnapshot(device.DeviceInfo.IPAddress)
	sendSuccessJSON(w, snap)
}

// HandleDeezerQueueAdd appends tracks to the end of the current queue.
func (app *WebApp) HandleDeezerQueueAdd(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}
	defer r.Body.Close()

	var req struct {
		Tracks []bmxpkg.QueueTrack `json:"tracks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Tracks) == 0 {
		app.sendError(w, "tracks must not be empty", http.StatusBadRequest)
		return
	}

	bmxpkg.AppendQueue(device.DeviceInfo.IPAddress, req.Tracks)
	snap := bmxpkg.GetQueueSnapshot(device.DeviceInfo.IPAddress)
	sendSuccessJSON(w, snap)
}

// HandleDeezerQueueStatus returns the queue snapshot.
func (app *WebApp) HandleDeezerQueueStatus(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	snap := bmxpkg.GetQueueSnapshot(device.DeviceInfo.IPAddress)
	sendSuccessJSON(w, snap)
}

// HandleDeezerQueueRemove removes one upcoming track by index (0 = first upcoming).
func (app *WebApp) HandleDeezerQueueRemove(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	idxStr := r.URL.Query().Get("index")
	if idxStr == "" {
		app.sendError(w, "missing required 'index' parameter", http.StatusBadRequest)
		return
	}

	index, err := strconv.Atoi(idxStr)
	if err != nil {
		app.sendError(w, "index query parameter must be an integer", http.StatusBadRequest)
		return
	}

	if err := bmxpkg.RemoveFromQueue(device.DeviceInfo.IPAddress, index); err != nil {
		app.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	snap := bmxpkg.GetQueueSnapshot(device.DeviceInfo.IPAddress)
	sendSuccessJSON(w, snap)
}

// HandleDeezerQueueStop stops playback and parks remaining tracks.
func (app *WebApp) HandleDeezerQueueStop(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	bmxpkg.StopQueue(device.DeviceInfo.IPAddress)
	snap := bmxpkg.GetQueueSnapshot(device.DeviceInfo.IPAddress)
	sendSuccessJSON(w, snap)
}

// HandleDeezerQueuePlay resumes from a parked queue.
func (app *WebApp) HandleDeezerQueuePlay(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	if err := bmxpkg.PlayQueue(device.DeviceInfo.IPAddress); err != nil {
		app.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	snap := bmxpkg.GetQueueSnapshot(device.DeviceInfo.IPAddress)
	sendSuccessJSON(w, snap)
}

// HandleDeezerQueueSkip advances to the next track immediately.
func (app *WebApp) HandleDeezerQueueSkip(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	bmxpkg.SkipTrack(device.DeviceInfo.IPAddress)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true})
}

// HandleDeezerQueueClear removes all upcoming tracks (and parked tracks if stopped).
func (app *WebApp) HandleDeezerQueueClear(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	bmxpkg.ClearUpcoming(device.DeviceInfo.IPAddress)
	snap := bmxpkg.GetQueueSnapshot(device.DeviceInfo.IPAddress)
	sendSuccessJSON(w, snap)
}

// HandleDeezerArtistDetails returns the full album list and top tracks for an artist.
// Uses concurrent goroutines to fetch albums and tracks in parallel.
func (app *WebApp) HandleDeezerArtistDetails(w http.ResponseWriter, r *http.Request) {
	artistID := chi.URLParam(r, "artistId")
	if artistID == "" {
		app.sendError(w, "artistId parameter is required", http.StatusBadRequest)
		return
	}

	var (
		albumsData *bmxpkg.DeezerArtistAlbumsResponse
		tracksData *bmxpkg.DeezerTrackListResponse
		errAlbums  error
		errTracks  error
		wg         sync.WaitGroup
	)

	// Fetch assets concurrently to minimize frontend blocking latency
	wg.Add(2)
	go func() {
		defer wg.Done()
		albumsData, errAlbums = bmxpkg.DeezerArtistAlbums(artistID)
	}()
	go func() {
		defer wg.Done()
		tracksData, errTracks = bmxpkg.DeezerArtistTopTracks(artistID)
	}()
	wg.Wait()

	if errAlbums != nil {
		app.sendError(w, fmt.Sprintf("failed to fetch albums: %v", errAlbums), http.StatusInternalServerError)
		return
	}
	if errTracks != nil {
		app.sendError(w, fmt.Sprintf("failed to fetch tracks: %v", errTracks), http.StatusInternalServerError)
		return
	}

	// Initialize slices with non-nil status to target JSON output as [] instead of null
	formattedAlbums := make([]map[string]interface{}, 0, len(albumsData.Data))
	for _, album := range albumsData.Data {
		formattedAlbums = append(formattedAlbums, map[string]interface{}{
			"id":           album.ID,
			"title":        album.Title,
			"cover_small":  album.CoverSmall,
			"cover_medium": album.CoverMed,
			"type":         "album",
		})
	}

	formattedTracks := make([]map[string]interface{}, 0, len(tracksData.Data))
	for _, track := range tracksData.Data {
		formattedTracks = append(formattedTracks, map[string]interface{}{
			"id":    track.ID,
			"title": track.Title,
			"album": map[string]string{
				"cover_small":  track.Album.CoverSmall,
				"cover_medium": track.Album.CoverMed,
			},
			"type": "track",
		})
	}

	sendSuccessJSON(w, map[string]interface{}{
		"albums": formattedAlbums,
		"tracks": formattedTracks,
	})
}

// HandleDeezerArtistTracklist returns an extended track list (~100 tracks) for an artist.
func (app *WebApp) HandleDeezerArtistTracklist(w http.ResponseWriter, r *http.Request) {
	artistID := chi.URLParam(r, "artistId")
	if artistID == "" {
		app.sendError(w, "artistId parameter is required", http.StatusBadRequest)
		return
	}

	tracksData, err := bmxpkg.DeezerArtistTracklist(artistID)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(tracksData.Data) == 0 {
		app.sendError(w, "No tracks found for this artist", http.StatusNotFound)
		return
	}

	formattedTracks := make([]map[string]interface{}, 0, len(tracksData.Data))
	for _, track := range tracksData.Data {
		formattedTracks = append(formattedTracks, map[string]interface{}{
			"id":    track.ID,
			"title": track.Title,
			"album": map[string]string{
				"cover_small":  track.Album.CoverSmall,
				"cover_medium": track.Album.CoverMed,
			},
			"type": "track",
		})
	}

	sendSuccessJSON(w, formattedTracks)
}

// HandleDeezerArtistRelated returns a list of artists similar to the given artist.
func (app *WebApp) HandleDeezerArtistRelated(w http.ResponseWriter, r *http.Request) {
	artistID := chi.URLParam(r, "artistId")
	if artistID == "" {
		app.sendError(w, "artistId parameter is required", http.StatusBadRequest)
		return
	}

	related, err := bmxpkg.DeezerArtistRelated(artistID)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if related == nil {
		related = []map[string]interface{}{}
	}

	sendSuccessJSON(w, related)
}

// HandleDeezerAlbumTracks returns all tracks for a given album.
func (app *WebApp) HandleDeezerAlbumTracks(w http.ResponseWriter, r *http.Request) {
	albumID := chi.URLParam(r, "albumId")
	if albumID == "" {
		app.sendError(w, "albumId parameter is required", http.StatusBadRequest)
		return
	}

	tracksData, err := bmxpkg.DeezerAlbumTracks(albumID)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	formattedTracks := make([]map[string]interface{}, 0, len(tracksData.Data))
	for _, track := range tracksData.Data {
		formattedTracks = append(formattedTracks, map[string]interface{}{
			"id":       track.ID,
			"title":    track.Title,
			"duration": track.Duration,
			"type":     "track",
		})
	}

	sendSuccessJSON(w, formattedTracks)
}

// SetupDeezerQueueBroadcaster registers the WebApp's WebSocket broadcast
// function, dispatching queue snapshot state updates concurrently and safely.
func (app *WebApp) SetupDeezerQueueBroadcaster() {
	bmxpkg.RegisterQueueBroadcaster(func(deviceIP string, snap bmxpkg.QueueSnapshot) {
		message := webtypes.WebSocketMessage{
			Type:     "deezer_queue",
			DeviceID: deviceIP,
			Data:     snap,
		}

		// Save a local snapshot of connections under a read lock immediately.
		// Performing write processing outside of locks prevents connection degradation backdoors.
		app.WSMutex.RLock()
		clients := make([]*websocket.Conn, 0, len(app.WSClients))
		for client := range app.WSClients {
			clients = append(clients, client)
		}
		app.WSMutex.RUnlock()

		var failed []*websocket.Conn
		for _, client := range clients {
			if err := client.WriteJSON(message); err != nil {
				log.Printf("[deezer-queue] WebSocket send error: %v", err)
				failed = append(failed, client)
			}
		}

		// Mutate map with exclusive write access to prevent race panics
		if len(failed) > 0 {
			app.WSMutex.Lock()
			for _, c := range failed {
				delete(app.WSClients, c)
				c.Close()
			}
			app.WSMutex.Unlock()
		}
	})
}
