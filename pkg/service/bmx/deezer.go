package bmx

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Base URL for the Deezer API.
const DefaultDeezerBaseURL = "https://api.deezer.com"

// DefaultClient is the default shared instance of the Deezer API wrapper.
var DefaultClient = NewClient(DefaultDeezerBaseURL, 10*time.Second)

// ============================================================================
// STRUCTS & MODELS
// ============================================================================

// DeezerError represents an error payload returned by Deezer's API.
// Note: Deezer often yields HTTP Status 200 but includes an error payload in the JSON.
type DeezerError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func (e *DeezerError) Error() string {
	return fmt.Sprintf("deezer error (%d): %s (type: %s)", e.Code, e.Message, e.Type)
}

// DeezerAPIResponse is a generic interface for payloads that might contain inline errors.
type DeezerAPIResponse struct {
	Error *DeezerError `json:"error,omitempty"`
}

// DeezerArtistAlbumsResponse holds a paginated list of artist albums.
type DeezerArtistAlbumsResponse struct {
	DeezerAPIResponse
	Data []struct {
		ID         int64  `json:"id"`
		Title      string `json:"title"`
		CoverSmall string `json:"cover_small"`
		CoverMed   string `json:"cover_medium"`
	} `json:"data"`
}

// DeezerTrackListResponse is used for artist top tracks, artist radio, and
// the extended artist tracklist.
type DeezerTrackListResponse struct {
	DeezerAPIResponse
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
	DeezerAPIResponse
	Data []struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		Duration int    `json:"duration"` // seconds
	} `json:"data"`
}

// SoundTouchSources XML structs for safe device parsing.
type SoundTouchSources struct {
	XMLName     xml.Name              `xml:"sources"`
	SourceItems []SoundTouchSourceItem `xml:"sourceItem"`
}

type SoundTouchSourceItem struct {
	Source        string `xml:"source,attr"`
	SourceAccount string `xml:"sourceAccount,attr"`
	Status        string `xml:"status,attr"`
}

// ============================================================================
// CLIENT MANAGER
// ============================================================================

// Client manages configurations and HTTP calls to the Deezer API.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient bootstraps a pristine Deezer HTTP Client.
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// ============================================================================
// API METHODS (CONTEXT-AWARE)
// ============================================================================

// DeezerSearch queries the Deezer search API and returns raw result maps.
func (c *Client) DeezerSearch(ctx context.Context, query string, searchType string) ([]map[string]interface{}, error) {
	validTypes := map[string]bool{"album": true, "artist": true, "track": true}
	endpoint := "search"
	if validTypes[searchType] {
		endpoint = "search/" + searchType
	}

	searchURL := fmt.Sprintf("%s/%s?q=%s", c.BaseURL, endpoint, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer search failed with HTTP status %d", resp.StatusCode)
	}

	var result struct {
		Data  []map[string]interface{} `json:"data"`
		Error *DeezerError             `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return result.Data, nil
}

// DeezerArtistAlbums retrieves all albums for an artist, handling pagination automatically.
func (c *Client) DeezerArtistAlbums(ctx context.Context, artistID string) (*DeezerArtistAlbumsResponse, error) {
	var final DeezerArtistAlbumsResponse
	index := 0
	const limit = 100 // Maximum batch size supported by Deezer

	for {
		apiURL := fmt.Sprintf("%s/artist/%s/albums?index=%d&limit=%d", c.BaseURL, artistID, index, limit)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.HTTPClient.Do(req)
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

		// Fail early on API errors returning HTTP 200
		if page.Error != nil {
			return nil, page.Error
		}

		final.Data = append(final.Data, page.Data...)

		// Stop pagination when matching final offsets or receiving empty items
		if len(page.Data) < limit {
			break
		}
		index += limit

		// Safety cap to prevent runaway memory on anomalous loops
		if index >= 1000 {
			break
		}
	}

	return &final, nil
}

// DeezerArtistTopTracks retrieves the standard top tracks of an artist.
func (c *Client) DeezerArtistTopTracks(ctx context.Context, artistID string) (*DeezerTrackListResponse, error) {
	return c.fetchTrackList(ctx, fmt.Sprintf("%s/artist/%s/top", c.BaseURL, artistID))
}

// DeezerArtistTracklist retrieves up to 100 top tracks for filling large device lists.
func (c *Client) DeezerArtistTracklist(ctx context.Context, artistID string) (*DeezerTrackListResponse, error) {
	return c.fetchTrackList(ctx, fmt.Sprintf("%s/artist/%s/top?limit=100", c.BaseURL, artistID))
}

// DeezerAlbumTracks retrieves all tracks for an album safely.
func (c *Client) DeezerAlbumTracks(ctx context.Context, albumID string) (*DeezerAlbumTracksResponse, error) {
	apiURL := fmt.Sprintf("%s/album/%s/tracks?limit=100", c.BaseURL, albumID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
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

	if data.Error != nil {
		return nil, data.Error
	}

	return &data, nil
}

// DeezerArtistRelated returns similar artists for recommendations.
func (c *Client) DeezerArtistRelated(ctx context.Context, artistID string) ([]map[string]interface{}, error) {
	apiURL := fmt.Sprintf("%s/artist/%s/related", c.BaseURL, artistID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer artist related failed with status %d", resp.StatusCode)
	}

	var result struct {
		Data  []map[string]interface{} `json:"data"`
		Error *DeezerError             `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return result.Data, nil
}

// fetchTrackList performs structured calls verifying inline API anomalies.
func (c *Client) fetchTrackList(ctx context.Context, apiURL string) (*DeezerTrackListResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer request failed with HTTP status %d", resp.StatusCode)
	}

	var data DeezerTrackListResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Error != nil {
		return nil, data.Error
	}

	return &data, nil
}

// DeezerSourceAccount queries local SoundTouch /sources over XML cleanly.
func (c *Client) DeezerSourceAccount(ctx context.Context, deviceIP string) string {
	sourcesURL := fmt.Sprintf("http://%s:8090/sources", deviceIP)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourcesURL, nil)
	if err != nil {
		return ""
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	var sources SoundTouchSources
	if err := xml.Unmarshal(body, &sources); err != nil {
		// Fallback logging or recovery can reside here
		return ""
	}

	for _, item := range sources.SourceItems {
		if strings.ToUpper(item.Source) == "DEEZER" {
			return item.SourceAccount
		}
	}

	return ""
}

// ============================================================================
// BACKWARD-COMPATIBLE WRAPPERS (PACKAGES-LEVEL)
// ============================================================================

// DeezerSearch functions exactly like the original API call.
func DeezerSearch(query string, searchType string) ([]map[string]interface{}, error) {
	return DefaultClient.DeezerSearch(context.Background(), query, searchType)
}

// DeezerArtistAlbums retrieves artist catalogs in drop-in structures.
func DeezerArtistAlbums(artistID string) (*DeezerArtistAlbumsResponse, error) {
	return DefaultClient.DeezerArtistAlbums(context.Background(), artistID)
}

// DeezerArtistTopTracks is backward-compatible with original calls.
func DeezerArtistTopTracks(artistID string) (*DeezerTrackListResponse, error) {
	return DefaultClient.DeezerArtistTopTracks(context.Background(), artistID)
}

// DeezerArtistTracklist functions with default configurations.
func DeezerArtistTracklist(artistID string) (*DeezerTrackListResponse, error) {
	return DefaultClient.DeezerArtistTracklist(context.Background(), artistID)
}

// DeezerAlbumTracks serves album listings seamlessly.
func DeezerAlbumTracks(albumID string) (*DeezerAlbumTracksResponse, error) {
	return DefaultClient.DeezerAlbumTracks(context.Background(), albumID)
}

// DeezerArtistRelated maps artist hierarchies cleanly.
func DeezerArtistRelated(artistID string) ([]map[string]interface{}, error) {
	return DefaultClient.DeezerArtistRelated(context.Background(), artistID)
}

// DeezerSourceAccount discovers accounts on the target device via safe XML models.
func DeezerSourceAccount(deviceIP string) string {
	return DefaultClient.DeezerSourceAccount(context.Background(), deviceIP)
}
