package soundtouchweb

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/stegerj/bose-soundtouch/pkg/models"
	bmxpkg "github.com/stegerj/bose-soundtouch/pkg/service/bmx"
	"github.com/stegerj/bose-soundtouch/pkg/service/soundtouchweb/webtypes"
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

// getDeviceAndSnapshot is a helper that retrieves a device and its queue snapshot.
// Returns the device IP, snapshot, and an error if the device is not found.
func (app *WebApp) getDeviceAndSnapshot(r *http.Request) (string, bmxpkg.QueueSnapshot, error) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		return "", bmxpkg.QueueSnapshot{}, fmt.Errorf("device not found")
	}
	snap := bmxpkg.GetQueueSnapshot(device.DeviceInfo.IPAddress)
	return device.DeviceInfo.IPAddress, snap, nil
}

// validateTracks validates that tracks are present and non-empty.
func validateTracks(tracks []bmxpkg.QueueTrack) error {
	if len(tracks) == 0 {
		return fmt.Errorf("tracks must not be empty")
	}
	return nil
}

// decodeTrackRequest decodes the request body into a tracks struct.
func decodeTrackRequest(r *http.Request) ([]bmxpkg.QueueTrack, error) {
	var req struct {
		Tracks []bmxpkg.QueueTrack `json:"tracks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, fmt.Errorf("invalid request body")
	}
	if err := validateTracks(req.Tracks); err != nil {
		return nil, err
	}
	return req.Tracks, nil
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
	deviceIP, _, err := app.getDeviceAndSnapshot(r)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusNotFound)
		return
	}
	defer r.Body.Close()

	tracks, err := decodeTrackRequest(r)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	bmxpkg.ReplaceQueue(deviceIP, tracks)
	snap := bmxpkg.GetQueueSnapshot(deviceIP)
	sendSuccessJSON(w, snap)
}

// HandleDeezerQueueAdd appends tracks to the end of the current queue.
func (app *WebApp) HandleDeezerQueueAdd(w http.ResponseWriter, r *http.Request) {
	deviceIP, _, err := app.getDeviceAndSnapshot(r)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusNotFound)
		return
	}
	defer r.Body.Close()

	tracks, err := decodeTrackRequest(r)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	bmxpkg.AppendQueue(deviceIP, tracks)
	snap := bmxpkg.GetQueueSnapshot(deviceIP)
	sendSuccessJSON(w, snap)
}

// HandleDeezerQueueStatus returns the queue snapshot.
func (app *WebApp) HandleDeezerQueueStatus(w http.ResponseWriter, r *http.Request) {
	_, snap, err := app.getDeviceAndSnapshot(r)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusNotFound)
		return
	}

	sendSuccessJSON(w, snap)
}

// HandleDeezerQueueRemove removes one upcoming track by index (0 = first upcoming).
func (app *WebApp) HandleDeezerQueueRemove(w http.ResponseWriter, r *http.Request) {
	deviceIP, _, err := app.getDeviceAndSnapshot(r)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusNotFound)
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

	if err := bmxpkg.RemoveFromQueue(deviceIP, index); err != nil {
		app.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	snap := bmxpkg.GetQueueSnapshot(deviceIP)
	sendSuccessJSON(w, snap)
}

// HandleDeezerQueueStop stops playback and parks remaining tracks.
func (app *WebApp) HandleDeezerQueueStop(w http.ResponseWriter, r *http.Request) {
	deviceIP, _, err := app.getDeviceAndSnapshot(r)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusNotFound)
		return
	}

	bmxpkg.StopQueue(deviceIP)
	snap := bmxpkg.GetQueueSnapshot(deviceIP)
	sendSuccessJSON(w, snap)
}

// HandleDeezerQueuePlay resumes from a parked queue.
func (app *WebApp) HandleDeezerQueuePlay(w http.ResponseWriter, r *http.Request) {
	deviceIP, _, err := app.getDeviceAndSnapshot(r)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := bmxpkg.PlayQueue(deviceIP); err != nil {
		app.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	snap := bmxpkg.GetQueueSnapshot(deviceIP)
	sendSuccessJSON(w, snap)
}

// HandleDeezerQueueSkip advances to the next track immediately.
func (app *WebApp) HandleDeezerQueueSkip(w http.ResponseWriter, r *http.Request) {
	deviceIP, _, err := app.getDeviceAndSnapshot(r)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusNotFound)
		return
	}

	bmxpkg.SkipTrack(deviceIP)
	sendSuccessJSON(w, nil)
}

// HandleDeezerQueueClear removes all upcoming tracks (and parked tracks if stopped).
func (app *WebApp) HandleDeezerQueueClear(w http.ResponseWriter, r *http.Request) {
	deviceIP, _, err := app.getDeviceAndSnapshot(r)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusNotFound)
		return
	}

	bmxpkg.ClearUpcoming(deviceIP)
	snap := bmxpkg.GetQueueSnapshot(deviceIP)
	sendSuccessJSON(w, snap)
}

// HandleDeezerPlayAlbum plays an album using native Deezer album mode (bypasses queue).
// This allows the album to be preset on the speaker and played natively.
func (app *WebApp) HandleDeezerPlayAlbum(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}
	defer r.Body.Close()

	var req struct {
		AlbumID int    `json:"albumId"`
		Name    string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.AlbumID == 0 {
		app.sendError(w, "albumId is required", http.StatusBadRequest)
		return
	}

	if device.Client == nil {
		app.sendError(w, "Device client not available", http.StatusInternalServerError)
		return
	}

	// Stop any existing queue playback for this device before playing new content
	bmxpkg.StopQueue(device.DeviceInfo.IPAddress)

	// Play album natively using the speaker's Deezer integration
	err := device.Client.SelectContentItem(&models.ContentItem{
		Source:        "DEEZER",
		Type:          "album",
		Location:      fmt.Sprintf("%d", req.AlbumID),
		ItemName:      req.Name,
		SourceAccount: bmxpkg.DeezerSourceAccount(device.DeviceInfo.IPAddress),
		IsPresetable:  true,
	})
	if err != nil {
		app.sendError(w, fmt.Sprintf("Failed to play album: %v", err), http.StatusInternalServerError)
		return
	}

	sendSuccessJSON(w, map[string]interface{}{"success": true})
}

// HandleDeezerPlayTrack plays a single track directly (bypasses queue).
func (app *WebApp) HandleDeezerPlayTrack(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}
	defer r.Body.Close()

	var req struct {
		TrackID int    `json:"trackId"`
		Title   string `json:"title"`
		Artist  string `json:"artist"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.TrackID == 0 {
		app.sendError(w, "trackId is required", http.StatusBadRequest)
		return
	}

	if device.Client == nil {
		app.sendError(w, "Device client not available", http.StatusInternalServerError)
		return
	}

	// Stop any existing queue playback for this device before playing new content
	bmxpkg.StopQueue(device.DeviceInfo.IPAddress)

	// Play track directly using the speaker's Deezer integration
	err := device.Client.SelectContentItem(&models.ContentItem{
		Source:        "DEEZER",
		Type:          "track",
		Location:      fmt.Sprintf("%d", req.TrackID),
		ItemName:      fmt.Sprintf("%s — %s", req.Title, req.Artist),
		SourceAccount: bmxpkg.DeezerSourceAccount(device.DeviceInfo.IPAddress),
		IsPresetable:  true,
	})
	if err != nil {
		app.sendError(w, fmt.Sprintf("Failed to play track: %v", err), http.StatusInternalServerError)
		return
	}

	sendSuccessJSON(w, map[string]interface{}{"success": true})
}

// HandleDeezerPlayArtist plays an artist's top tracks directly (bypasses queue).
func (app *WebApp) HandleDeezerPlayArtist(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}
	defer r.Body.Close()

	var req struct {
		ArtistID int    `json:"artistId"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.ArtistID == 0 {
		app.sendError(w, "artistId is required", http.StatusBadRequest)
		return
	}

	if device.Client == nil {
		app.sendError(w, "Device client not available", http.StatusInternalServerError)
		return
	}

	// Stop any existing queue playback for this device before playing new content
	bmxpkg.StopQueue(device.DeviceInfo.IPAddress)

	// Play artist directly using the speaker's Deezer integration
	err := device.Client.SelectContentItem(&models.ContentItem{
		Source:        "DEEZER",
		Type:          "artist",
		Location:      fmt.Sprintf("%d", req.ArtistID),
		ItemName:      req.Name,
		SourceAccount: bmxpkg.DeezerSourceAccount(device.DeviceInfo.IPAddress),
		IsPresetable:  true,
	})
	if err != nil {
		app.sendError(w, fmt.Sprintf("Failed to play artist: %v", err), http.StatusInternalServerError)
		return
	}

	sendSuccessJSON(w, map[string]interface{}{"success": true})
}

// HandleDeezerQueueAutoStart enables or disables auto-start for the queue.
// When enabled, the queue will automatically start when the device becomes idle.
func (app *WebApp) HandleDeezerQueueAutoStart(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}
	defer r.Body.Close()

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	bmxpkg.SetAutoStart(device.DeviceInfo.IPAddress, req.Enabled)
	sendSuccessJSON(w, map[string]interface{}{"success": true, "enabled": req.Enabled})
}

// HandleDeezerQueueAutoStartGet returns the current auto-start state for a device.
func (app *WebApp) HandleDeezerQueueAutoStartGet(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	enabled := bmxpkg.GetAutoStart(device.DeviceInfo.IPAddress)
	sendSuccessJSON(w, map[string]interface{}{"success": true, "enabled": enabled})
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
