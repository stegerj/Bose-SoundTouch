package bmx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Dominio ufficiale dell'API di Deezer
var deezerBaseURL = "https://api.deezer.com"

// ============================================================================
// STRUTTURE DATI (STRUC) PER LE RISPOSTE API
// ============================================================================

// DeezerArtistAlbumsResponse definisce la struttura per la lista degli album
type DeezerArtistAlbumsResponse struct {
	Data []struct {
		ID         int64  `json:"id"`
		Title      string `json:"title"`
		CoverSmall string `json:"cover_small"`
		CoverMed   string `json:"cover_medium"`
	} `json:"data"`
}

// DeezerArtistTracksResponse definisce la struttura per le tracce top di un artista
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

// DeezerAlbumTracksResponse definisce la struttura per le tracce di un singolo album
type DeezerAlbumTracksResponse struct {
	Data []struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		Duration int    `json:"duration"` // Durata in secondi
	} `json:"data"`
}

// DeezerArtistRadioResponse definisce la struttura per i brani estratti dalla radio dell'artista
type DeezerArtistRadioResponse struct {
	Data []struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
		Album struct {
			CoverSmall string `json:"cover_small"`
			CoverMed   string `json:"cover_medium"`
		} `json:"album"`
	} `json:"data"`
}


// ============================================================================
// FUNZIONI API DEEZER
// ============================================================================

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

// DeezerArtistAlbums recupera TUTTI gli album di un artista gestendo la paginazione automatica.
func DeezerArtistAlbums(artistID string) (*DeezerArtistAlbumsResponse, error) {
	var finalResult DeezerArtistAlbumsResponse
	index := 0
	limit := 100 // Massimo consentito da Deezer per singola richiesta

	for {
		apiURL := fmt.Sprintf("%s/artist/%s/albums?index=%d&limit=%d", deezerBaseURL, artistID, index, limit)
		resp, err := http.Get(apiURL)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("deezer artist albums failed with status %d", resp.StatusCode)
		}

		var pageData DeezerArtistAlbumsResponse
		err = json.NewDecoder(resp.Body).Decode(&pageData)
		_ = resp.Body.Close() // Chiude il body immediatamente ad ogni iterazione del ciclo
		if err != nil {
			return nil, err
		}

		// Unisce gli elementi della pagina corrente al risultato finale
		finalResult.Data = append(finalResult.Data, pageData.Data...)

		// Se gli elementi ricevuti sono inferiori al limite richiesto, la paginazione è terminata
		if len(pageData.Data) < limit {
			break
		}
		index += limit
	}

	return &finalResult, nil
}

// DeezerArtistTopTracks recupera le tracce più popolari di un artista direttamente da Deezer.
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

// DeezerAlbumTracks recupera la lista di tutte le tracce contenute in un determinato album.
func DeezerAlbumTracks(albumID string) (*DeezerAlbumTracksResponse, error) {
	// Usando limit=100 evitiamo il ciclo di paginazione, coprendo quasi la totalità degli album commerciali
	apiURL := fmt.Sprintf("%s/album/%s/tracks?limit=100", deezerBaseURL, albumID)
	resp, err := http.Get(apiURL)
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

// DeezerArtistRadio recupera i brani della radio di un artista direttamente da Deezer
func DeezerArtistRadio(artistID string) (*DeezerArtistRadioResponse, error) {
	apiURL := fmt.Sprintf("%s/artist/%s/radio", deezerBaseURL, artistID)
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deezer artist radio failed with status %d", resp.StatusCode)
	}

	var data DeezerArtistRadioResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}
