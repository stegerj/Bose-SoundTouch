package soundtouchweb

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/websocket" // same package used by BroadcastDeviceList — adjust path if different
	"github.com/gesellix/bose-soundtouch/pkg/models"
	bmxpkg "github.com/gesellix/bose-soundtouch/pkg/service/bmx"
	"github.com/gesellix/bose-soundtouch/pkg/service/soundtouchweb/webtypes"
	"github.com/go-chi/chi/v5"
)

// HandleDeezerSearch handles Deezer search requests.
func (app *WebApp) HandleDeezerSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	// Ripristinato il parsing originale tramite Chi per compatibilità con il frontend
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

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: rawItems})
}

// HandlePlayDeezer plays a Deezer track, album, or artist on a specific SoundTouch device.
func (app *WebApp) HandlePlayDeezer(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}
	defer r.Body.Close()

	var req struct {
		Location json.RawMessage `json:"location"`
		Name     string          `json:"itemName"`
		Type     string          `json:"type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var locationStr string
	if len(req.Location) > 0 {
		locationStr = string(req.Location)
		if len(locationStr) > 1 && locationStr[0] == '"' && locationStr[len(locationStr)-1] == '"' {
			locationStr = locationStr[1 : len(locationStr)-1]
		}
	}

	if locationStr == "" {
		app.sendError(w, "Location/ID is required", http.StatusBadRequest)
		return
	}

	boseType := req.Type
	if boseType == "" {
		boseType = "album"
	}
	if boseType == "artist" {
		boseType = "artistradio"
	}

	// Recupero dell'account con il vecchio metodo string-matching (Infallibile)
	sourceAccount := app.extractDeezerAccount(device.DeviceInfo.IPAddress)

	// Fallback post-spegnimento cloud Bose (Giugno 2026)
	// Se la cassa restituisce un XML vuoto o privo di account, forziamo un ID fittizio
	if sourceAccount == "" {
		log.Printf("[Deezer] No native account found on %s. Applying post-cloud dummy identifier.", device.DeviceInfo.IPAddress)
		sourceAccount = "12345678"
	}

	contentItem := &models.ContentItem{
		Source:        "DEEZER",
		Type:          boseType,
		Location:      locationStr,
		ItemName:      req.Name,
		SourceAccount: sourceAccount,
		IsPresetable:  true,
	}

	if err := device.Client.SelectContentItem(contentItem); err != nil {
		app.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: map[string]string{"message": "Playing " + req.Name}})
}

// HandleDeezerQueueReplace replaces the current queue with the supplied
// tracklist and starts playing immediately. This is the ▶ play action —
// the old queue (if any) is discarded.
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
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: snap})
}

// HandleDeezerQueueAdd appends tracks to the end of the current queue. If
// nothing is playing it starts immediately. This is the + add action.
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
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: snap})
}

// HandleDeezerQueueStatus returns the queue snapshot: currently-playing
// track (nil when idle) plus the upcoming tracks.
func (app *WebApp) HandleDeezerQueueStatus(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	snap := bmxpkg.GetQueueSnapshot(device.DeviceInfo.IPAddress)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: snap})
}

// HandleDeezerQueueRemove removes one upcoming track by index (0 = first
// upcoming, not the currently-playing one).
func (app *WebApp) HandleDeezerQueueRemove(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	index, err := strconv.Atoi(r.URL.Query().Get("index"))
	if err != nil {
		app.sendError(w, "index query parameter must be an integer", http.StatusBadRequest)
		return
	}
	if err := bmxpkg.RemoveFromQueue(device.DeviceInfo.IPAddress, index); err != nil {
		app.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	snap := bmxpkg.GetQueueSnapshot(device.DeviceInfo.IPAddress)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: snap})
}

// HandleDeezerQueueStop stops playback and parks the remaining tracks so
// HandleDeezerQueuePlay can resume them later.
func (app *WebApp) HandleDeezerQueueStop(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}
	bmxpkg.StopQueue(device.DeviceInfo.IPAddress)
	snap := bmxpkg.GetQueueSnapshot(device.DeviceInfo.IPAddress)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: snap})
}

// HandleDeezerQueuePlay resumes from a parked (stopped) queue.
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
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: snap})
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

// HandleDeezerQueueClear removes all upcoming tracks (and the parked list if
// stopped). The currently-playing track (if any) continues.
func (app *WebApp) HandleDeezerQueueClear(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}
	bmxpkg.ClearUpcoming(device.DeviceInfo.IPAddress)
	snap := bmxpkg.GetQueueSnapshot(device.DeviceInfo.IPAddress)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: snap})
}

// HandleDeezerArtistDetails returns the full album list and top tracks for an artist.
func (app *WebApp) HandleDeezerArtistDetails(w http.ResponseWriter, r *http.Request) {
	artistID := chi.URLParam(r, "artistId")
	if artistID == "" {
		app.sendError(w, "artistId parameter is required", http.StatusBadRequest)
		return
	}

	albumsData, err := bmxpkg.DeezerArtistAlbums(artistID)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tracksData, err := bmxpkg.DeezerArtistTopTracks(artistID)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var formattedAlbums []map[string]interface{}
	for _, album := range albumsData.Data {
		formattedAlbums = append(formattedAlbums, map[string]interface{}{
			"id":           album.ID,
			"title":        album.Title,
			"cover_small":  album.CoverSmall,
			"cover_medium": album.CoverMed,
			"type":         "album",
		})
	}

	var formattedTracks []map[string]interface{}
	for _, track := range tracksData.Data {
		formattedTracks = append(formattedTracks, map[string]interface{}{
			"id":      track.ID,
			"title":   track.Title,
			"album": map[string]string{
				"cover_small":  track.Album.CoverSmall,
				"cover_medium": track.Album.CoverMed,
			},
			"type": "track",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"albums": formattedAlbums,
			"tracks": formattedTracks,
		},
	})
}

// HandleDeezerArtistAlbums returns the full album list for an artist (used
// to populate the "Albums" section of the artist drill-down view).
func (app *WebApp) HandleDeezerArtistAlbums(w http.ResponseWriter, r *http.Request) {
	artistID := chi.URLParam(r, "artistId")
	if artistID == "" {
		app.sendError(w, "artistId parameter is required", http.StatusBadRequest)
		return
	}

	albumsData, err := bmxpkg.DeezerArtistAlbums(artistID)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var formattedAlbums []map[string]interface{}
	for _, album := range albumsData.Data {
		formattedAlbums = append(formattedAlbums, map[string]interface{}{
			"id":           album.ID,
			"title":        album.Title,
			"cover_small":  album.CoverSmall,
			"cover_medium": album.CoverMed,
			"type":         "album",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: formattedAlbums})
}

// HandleDeezerArtistTracklist returns an extended track list (up to ~100
// tracks) for an artist — the dedicated "Tracklist" feature, since
// HandleDeezerArtistDetails's top-tracks list is only ~25 tracks.
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

	var formattedTracks []map[string]interface{}
	for _, track := range tracksData.Data {
		formattedTracks = append(formattedTracks, map[string]interface{}{
			"id":      track.ID,
			"title":   track.Title,
			"album": map[string]string{
				"cover_small":  track.Album.CoverSmall,
				"cover_medium": track.Album.CoverMed,
			},
			"type": "track",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: formattedTracks})
}

// HandleDeezerArtistRadio returns the track list for an artist's radio.
func (app *WebApp) HandleDeezerArtistRadio(w http.ResponseWriter, r *http.Request) {
	artistID := chi.URLParam(r, "artistId")
	if artistID == "" {
		app.sendError(w, "artistId parameter is required", http.StatusBadRequest)
		return
	}

	radioData, err := bmxpkg.DeezerArtistRadio(artistID)
	if err != nil {
		app.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(radioData.Data) == 0 {
		app.sendError(w, "No tracks found in this artist radio", http.StatusNotFound)
		return
	}

	var formattedTracks []map[string]interface{}
	for _, track := range radioData.Data {
		formattedTracks = append(formattedTracks, map[string]interface{}{
			"id":      track.ID,
			"title":   track.Title,
			"album": map[string]string{
				"cover_small":  track.Album.CoverSmall,
				"cover_medium": track.Album.CoverMed,
			},
			"type": "track",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: formattedTracks})
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

	var formattedTracks []map[string]interface{}
	for _, track := range tracksData.Data {
		formattedTracks = append(formattedTracks, map[string]interface{}{
			"id":       track.ID,
			"title":    track.Title,
			"duration": track.Duration,
			"type":     "track",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: formattedTracks})
}

// extractDeezerAccount reads the device's currently configured Deezer
// source account. The actual XML probing lives in bmxpkg.DeezerSourceAccount
// so the queue (deezer_queue.go) and this classic single-item play path
// share one implementation instead of two copies drifting apart.
func (app *WebApp) extractDeezerAccount(deviceIP string) string {
	return bmxpkg.DeezerSourceAccount(deviceIP)
}

// SetupDeezerQueueBroadcaster registers the WebApp's WebSocket broadcast
// function with the bmx queue package. Call this once at startup (from
// MountWeb) so every queue state change is pushed to all connected clients
// as a "deezer_queue" message, eliminating UI polling entirely.
func (app *WebApp) SetupDeezerQueueBroadcaster() {
	bmxpkg.RegisterQueueBroadcaster(func(deviceIP string, snap bmxpkg.QueueSnapshot) {
		message := webtypes.WebSocketMessage{
			Type:     "deezer_queue",
			DeviceID: deviceIP,
			Data:     snap,
		}

		app.WSMutex.RLock()
		defer app.WSMutex.RUnlock()

		var failed []*websocket.Conn
		for client := range app.WSClients {
			if err := client.WriteJSON(message); err != nil {
				log.Printf("[deezer-queue] WebSocket send error: %v", err)
				failed = append(failed, client)
			}
		}
		for _, c := range failed {
			delete(app.WSClients, c)
			c.Close()
		}
	})
}

func deezerUnquote(raw json.RawMessage) string {
	str := string(raw)
	if strings.HasPrefix(str, `"`) && strings.HasSuffix(str, `"`) {
		return str[1 : len(str)-1]
	}
	return str
}
