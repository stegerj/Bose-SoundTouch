package soundtouchweb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

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

// HandleDeezerQueue accepts a tracklist and immediately starts sequential
// playback on the device, replacing whatever "hidden" quick-play queue was
// running before. Used for "continue playing the rest of this album /
// artist tracklist" actions. It does not touch the persistent visible queue
// (see HandleDeezerQueueAdd / HandleDeezerQueuePlay below).
func (app *WebApp) HandleDeezerQueue(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")
	device, exists := app.GetDevice(deviceID)
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	var req struct {
		Tracks []bmxpkg.QueueTrack `json:"tracks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Tracks) == 0 {
		app.sendError(w, "Tracks must not be empty", http.StatusBadRequest)
		return
	}

	bmxpkg.StartQueue(device.DeviceInfo.IPAddress, req.Tracks)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{
		Success: true,
		Data:    map[string]any{"queued": len(req.Tracks), "device_id": deviceID},
	})
}

// HandleDeezerQueueStop cancels any active queue (hidden or visible) on a
// device. If the visible queue was the one playing, its track list is kept
// so it can be replayed from the top via HandleDeezerQueuePlay.
func (app *WebApp) HandleDeezerQueueStop(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "id")
	device, exists := app.GetDevice(deviceID)
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	bmxpkg.StopQueue(device.DeviceInfo.IPAddress)
	w.WriteHeader(http.StatusNoContent)
}

// HandleDeezerQueueStatus returns the current state of the device's visible
// queue (tracks, whether it's playing, current position).
func (app *WebApp) HandleDeezerQueueStatus(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	snapshot := bmxpkg.GetVisibleQueueSnapshot(device.DeviceInfo.IPAddress)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: snapshot})
}

// HandleDeezerQueueAdd appends tracks to the device's persistent, visible
// queue (the UI's "+" add-to-queue buttons). Unlike HandleDeezerQueue, this
// only stages tracks — playback starts when HandleDeezerQueuePlay is called,
// or continues seamlessly if the visible queue is already playing.
func (app *WebApp) HandleDeezerQueueAdd(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	var req struct {
		Tracks []bmxpkg.QueueTrack `json:"tracks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Tracks) == 0 {
		app.sendError(w, "Tracks must not be empty", http.StatusBadRequest)
		return
	}

	bmxpkg.AddToVisibleQueue(device.DeviceInfo.IPAddress, req.Tracks)

	snapshot := bmxpkg.GetVisibleQueueSnapshot(device.DeviceInfo.IPAddress)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: snapshot})
}

// HandleDeezerQueuePlay starts (or restarts, from the top) playback of the
// device's visible queue.
func (app *WebApp) HandleDeezerQueuePlay(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	if err := bmxpkg.PlayVisibleQueue(device.DeviceInfo.IPAddress); err != nil {
		app.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	snapshot := bmxpkg.GetVisibleQueueSnapshot(device.DeviceInfo.IPAddress)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: snapshot})
}

// HandleDeezerQueueRemove removes a single track (by index) from the
// device's visible queue buffer.
func (app *WebApp) HandleDeezerQueueRemove(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	index, err := strconv.Atoi(r.URL.Query().Get("index"))
	if err != nil {
		app.sendError(w, "index parameter must be an integer", http.StatusBadRequest)
		return
	}

	if err := bmxpkg.RemoveFromVisibleQueue(device.DeviceInfo.IPAddress, index); err != nil {
		app.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	snapshot := bmxpkg.GetVisibleQueueSnapshot(device.DeviceInfo.IPAddress)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: snapshot})
}

// HandleDeezerQueueClear empties the device's visible queue buffer.
func (app *WebApp) HandleDeezerQueueClear(w http.ResponseWriter, r *http.Request) {
	device, exists := app.GetDevice(chi.URLParam(r, "id"))
	if !exists {
		app.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	bmxpkg.ClearVisibleQueue(device.DeviceInfo.IPAddress)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: bmxpkg.VisibleQueueSnapshot{}})
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
			"preview": track.Preview,
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
			"preview": track.Preview,
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
			"preview": track.Preview,
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
			"preview":  track.Preview,
			"type":     "track",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webtypes.APIResponse{Success: true, Data: formattedTracks})
}

// extractDeezerAccount esegue lo split di stringa originale che leggeva correttamente l'XML
func (app *WebApp) extractDeezerAccount(deviceIP string) string {
	sourcesURL := fmt.Sprintf("http://%s:8090/sources", deviceIP)
	resp, err := http.Get(sourcesURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, resp.Body)
	xmlStr := buf.String()

	if strings.Contains(xmlStr, `source="DEEZER"`) {
		parts := strings.Split(xmlStr, `source="DEEZER"`)
		if len(parts) > 1 {
			subParts := strings.Split(parts[1], `sourceAccount="`)
			if len(subParts) > 1 {
				emailParts := strings.Split(subParts[1], `"`)
				if len(emailParts) > 0 {
					return emailParts[0]
				}
			}
		}
	}
	return ""
}

func deezerUnquote(raw json.RawMessage) string {
	str := string(raw)
	if strings.HasPrefix(str, `"`) && strings.HasSuffix(str, `"`) {
		return str[1 : len(str)-1]
	}
	return str
}
