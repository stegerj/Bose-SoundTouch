package bmx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// FIXED: Ora punta al dominio API corretto anziché a quello web standard
var deezerBaseURL = "https://api.deezer.com"

// DeezerSearch interroga direttamente Deezer e restituisce l'array di mappe JSON grezze.
func DeezerSearch(query string, searchType string) ([]map[string]interface{}, error) {
	endpoint := "search"
	if searchType == "album" || searchType == "artist" || searchType == "track" {
		endpoint = "search/" + searchType
	}

	searchURL := fmt.Sprintf("%s/%s?q=%s", deezerBaseURL, endpoint, url.QueryEscape(query))

	resp, err := http.Get(searchURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer search failed with status %d", resp.StatusCode)
	}

	var deezerResult struct {
		Data []map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&deezerResult); err != nil {
		return nil, err
	}

	return deezerResult.Data, nil
}

type DeezerArtistAlbumsResponse struct {
	Data []struct {
		ID         int64  `json:"id"`
		Title      string `json:"title"`
		CoverSmall string `json:"cover_small"`
		CoverMed   string `json:"cover_medium"`
	} `json:"data"`
}

type DeezerArtistTracksResponse struct {
	Data []struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Album struct {
			CoverSmall string `json:"cover_small"`
			CoverMed   string `json:"cover_medium"`
		} `json:"album"`
	} `json:"data"`
}

// DeezerArtistAlbums recupera la lista degli album di un artista direttamente da Deezer
func DeezerArtistAlbums(artistID string) (*DeezerArtistAlbumsResponse, error) {
	apiURL := fmt.Sprintf("%s/artist/%s/albums", deezerBaseURL, artistID)
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer artist albums failed with status %d", resp.StatusCode)
	}

	var data DeezerArtistAlbumsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

// DeezerArtistTopTracks recupera le tracce più popolari di un artista direttamente da Deezer
func DeezerArtistTopTracks(artistID string) (*DeezerArtistTracksResponse, error) {
	apiURL := fmt.Sprintf("%s/artist/%s/top", deezerBaseURL, artistID)
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer artist top tracks failed with status %d", resp.StatusCode)
	}

	var data DeezerArtistTracksResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}
