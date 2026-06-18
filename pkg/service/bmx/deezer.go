package bmx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Base URL for the Deezer API.
const deezerBaseURL = "https://api.deezer.com"

// httpClient is shared across all Deezer API calls with a sensible timeout.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// ============================================================================
// RESPONSE TYPES
// ============================================================================

// DeezerArtistAlbumsResponse holds a paginated list of artist albums.
type DeezerArtistAlbumsResponse struct {
	Data []struct {
		ID         int64  `json:"id"`
		Title      string `json:"title"`
		CoverSmall string `json:"cover_small"`
		CoverMed   string `json:"cover_medium"`
	} `json:"data"`
}

// DeezerTrackListResponse is used for artist top tracks, artist radio, and
// the extended artist tracklist, since all three endpoints return the same
// shape.
type DeezerTrackListResponse struct {
	Data []struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Album struct {
			CoverSmall string `json:"cover_small"`
			CoverMed   string `json:"cover_medium"`
		} `json:"album"`
	} `json:"data"`
}

// DeezerAlbumTracksResponse holds the tracks for a single album.
type DeezerAlbumTracksResponse struct {
	Data []struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		Duration int    `json:"duration"` // seconds
	} `json:"data"`
}

// ============================================================================
// API FUNCTIONS
// ============================================================================

// DeezerSearch queries the Deezer search API and returns raw result maps.
// searchType must be one of "album", "artist", or "track"; anything else
// falls back to the generic search endpoint.
func DeezerSearch(query string, searchType string) ([]map[string]interface{}, error) {
	validTypes := map[string]bool{"album": true, "artist": true, "track": true}
	endpoint := "search"
	if validTypes[searchType] {
		endpoint = "search/" + searchType
	}

	searchURL := fmt.Sprintf("%s/%s?q=%s", deezerBaseURL, endpoint, url.QueryEscape(query))

	resp, err := httpClient.Get(searchURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer search failed with status %d", resp.StatusCode)
	}

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// DeezerArtistAlbums retrieves all albums for an artist, handling pagination
// automatically.
func DeezerArtistAlbums(artistID string) (*DeezerArtistAlbumsResponse, error) {
	var final DeezerArtistAlbumsResponse
	index := 0
	const limit = 100 // maximum Deezer allows per request

	for {
		apiURL := fmt.Sprintf("%s/artist/%s/albums?index=%d&limit=%d", deezerBaseURL, artistID, index, limit)
		resp, err := httpClient.Get(apiURL)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("deezer artist albums failed with status %d", resp.StatusCode)
		}

		var page DeezerArtistAlbumsResponse
		err = json.NewDecoder(resp.Body).Decode(&page)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}

		final.Data = append(final.Data, page.Data...)

		if len(page.Data) < limit {
			break
		}
		index += limit
	}

	return &final, nil
}

// DeezerArtistTopTracks retrieves the most popular tracks for an artist
// (Deezer's default page size, roughly two dozen tracks).
func DeezerArtistTopTracks(artistID string) (*DeezerTrackListResponse, error) {
	return fetchTrackList(fmt.Sprintf("%s/artist/%s/top", deezerBaseURL, artistID))
}

// DeezerArtistTracklist retrieves an extended track list for an artist (up
// to 100 tracks). DeezerArtistTopTracks's default page is far too short to
// back a "queue everything by this artist" feature, so this requests a much
// higher limit explicitly.
func DeezerArtistTracklist(artistID string) (*DeezerTrackListResponse, error) {
	return fetchTrackList(fmt.Sprintf("%s/artist/%s/top?limit=100", deezerBaseURL, artistID))
}

// DeezerArtistRadio retrieves the artist radio tracks.
func DeezerArtistRadio(artistID string) (*DeezerTrackListResponse, error) {
	return fetchTrackList(fmt.Sprintf("%s/artist/%s/radio", deezerBaseURL, artistID))
}

// DeezerAlbumTracks retrieves all tracks for an album.
// limit=100 covers virtually all commercial albums without needing pagination.
func DeezerAlbumTracks(albumID string) (*DeezerAlbumTracksResponse, error) {
	apiURL := fmt.Sprintf("%s/album/%s/tracks?limit=100", deezerBaseURL, albumID)
	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer album tracks failed with status %d", resp.StatusCode)
	}

	var data DeezerAlbumTracksResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

// fetchTrackList is a shared helper for endpoints that return DeezerTrackListResponse.
func fetchTrackList(apiURL string) (*DeezerTrackListResponse, error) {
	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer request to %s failed with status %d", apiURL, resp.StatusCode)
	}

	var data DeezerTrackListResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

// DeezerSourceAccount returns the device's currently configured Deezer
// source account (the account that was logged into Deezer directly on the
// speaker), read from its local /sources endpoint. Playback is by Deezer
// catalog ID — both the classic single-item play and the queue mechanism
// resolve and stream audio device-side via this account, the same way.
// Returns "" if no account is found; callers apply their own fallback.
func DeezerSourceAccount(deviceIP string) string {
	sourcesURL := fmt.Sprintf("http://%s:8090/sources", deviceIP)
	resp, err := httpClient.Get(sourcesURL)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

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
