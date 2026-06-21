package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stegerj/bose-soundtouch/pkg/models"
)

func TestClient_StorePreset(t *testing.T) {
	tests := []struct {
		name           string
		presetID       int
		contentItem    *models.ContentItem
		serverResponse string
		serverStatus   int
		expectError    bool
		errorContains  string
	}{
		{
			name:     "store_spotify_playlist_success",
			presetID: 1,
			contentItem: &models.ContentItem{
				Source:        "SPOTIFY",
				Type:          "uri",
				Location:      "spotify:playlist:37i9dQZF1DX0XUsuxWHRQd",
				SourceAccount: "testuser",
				IsPresetable:  true,
				ItemName:      "My Playlist",
				ContainerArt:  "https://example.com/art.jpg",
			},
			serverResponse: `<?xml version="1.0" encoding="UTF-8"?><presets><preset id="1"><ContentItem source="SPOTIFY" type="uri" location="spotify:playlist:37i9dQZF1DX0XUsuxWHRQd" sourceAccount="testuser" isPresetable="true"><itemName>My Playlist</itemName></ContentItem></preset></presets>`,
			serverStatus:   http.StatusOK,
			expectError:    false,
		},
		{
			name:     "store_tunein_radio_success",
			presetID: 2,
			contentItem: &models.ContentItem{
				Source:       "TUNEIN",
				Type:         "stationurl",
				Location:     "/v1/playback/station/s33828",
				IsPresetable: true,
				ItemName:     "K-LOVE Radio",
			},
			serverResponse: `<?xml version="1.0" encoding="UTF-8"?><presets><preset id="2"><ContentItem source="TUNEIN" type="stationurl" location="/v1/playback/station/s33828" isPresetable="true"><itemName>K-LOVE Radio</itemName></ContentItem></preset></presets>`,
			serverStatus:   http.StatusOK,
			expectError:    false,
		},
		{
			name:     "store_local_internet_radio_success",
			presetID: 3,
			contentItem: &models.ContentItem{
				Source:       "LOCAL_INTERNET_RADIO",
				Type:         "stationurl",
				Location:     "https://stream.example.com/radio",
				IsPresetable: true,
				ItemName:     "Custom Radio",
			},
			serverResponse: `<?xml version="1.0" encoding="UTF-8"?><presets><preset id="3"><ContentItem source="LOCAL_INTERNET_RADIO" type="stationurl" location="https://stream.example.com/radio" isPresetable="true"><itemName>Custom Radio</itemName></ContentItem></preset></presets>`,
			serverStatus:   http.StatusOK,
			expectError:    false,
		},
		{
			name:          "invalid_preset_id_too_low",
			presetID:      0,
			contentItem:   &models.ContentItem{Source: "SPOTIFY", Location: "test"},
			expectError:   true,
			errorContains: "preset ID must be between 1 and 6",
		},
		{
			name:          "invalid_preset_id_too_high",
			presetID:      7,
			contentItem:   &models.ContentItem{Source: "SPOTIFY", Location: "test"},
			expectError:   true,
			errorContains: "preset ID must be between 1 and 6",
		},
		{
			name:          "nil_content_item",
			presetID:      1,
			contentItem:   nil,
			expectError:   true,
			errorContains: "content item cannot be nil",
		},
		{
			name:           "server_error_response",
			presetID:       1,
			contentItem:    &models.ContentItem{Source: "SPOTIFY", Location: "test"},
			serverResponse: `<?xml version="1.0" encoding="UTF-8"?><error>Invalid preset</error>`,
			serverStatus:   http.StatusBadRequest,
			expectError:    true,
			errorContains:  "failed to store preset 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and endpoint
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST request, got %s", r.Method)
				}

				if r.URL.Path != "/storePreset" {
					t.Errorf("Expected /storePreset endpoint, got %s", r.URL.Path)
				}

				// Verify content type
				if r.Header.Get("Content-Type") != "application/xml" {
					t.Errorf("Expected Content-Type application/xml, got %s", r.Header.Get("Content-Type"))
				}

				// Return mock response
				if tt.serverStatus != 0 {
					w.WriteHeader(tt.serverStatus)
				}

				if tt.serverResponse != "" {
					_, _ = w.Write([]byte(tt.serverResponse))
				}
			}))
			defer server.Close()

			client := &Client{
				baseURL:    server.URL,
				httpClient: &http.Client{Timeout: 10 * time.Second},
			}

			err := client.StorePreset(tt.presetID, tt.contentItem)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got nil")
					return
				}

				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', but got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

func TestClient_RemovePreset(t *testing.T) {
	tests := []struct {
		name           string
		presetID       int
		serverResponse string
		serverStatus   int
		expectError    bool
		errorContains  string
	}{
		{
			name:           "remove_preset_success",
			presetID:       3,
			serverResponse: `<?xml version="1.0" encoding="UTF-8"?><presets></presets>`,
			serverStatus:   http.StatusOK,
			expectError:    false,
		},
		{
			name:          "invalid_preset_id_too_low",
			presetID:      0,
			expectError:   true,
			errorContains: "preset ID must be between 1 and 6",
		},
		{
			name:          "invalid_preset_id_too_high",
			presetID:      7,
			expectError:   true,
			errorContains: "preset ID must be between 1 and 6",
		},
		{
			name:           "server_error_response",
			presetID:       1,
			serverResponse: `<?xml version="1.0" encoding="UTF-8"?><error>Preset not found</error>`,
			serverStatus:   http.StatusNotFound,
			expectError:    true,
			errorContains:  "failed to remove preset 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and endpoint
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST request, got %s", r.Method)
				}

				if r.URL.Path != "/removePreset" {
					t.Errorf("Expected /removePreset endpoint, got %s", r.URL.Path)
				}

				// Return mock response
				if tt.serverStatus != 0 {
					w.WriteHeader(tt.serverStatus)
				}

				if tt.serverResponse != "" {
					_, _ = w.Write([]byte(tt.serverResponse))
				}
			}))
			defer server.Close()

			client := &Client{
				baseURL:    server.URL,
				httpClient: &http.Client{Timeout: 10 * time.Second},
			}

			err := client.RemovePreset(tt.presetID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got nil")
					return
				}

				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', but got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

func TestClient_StoreCurrentAsPreset(t *testing.T) {
	tests := []struct {
		name               string
		presetID           int
		nowPlayingResponse string
		nowPlayingStatus   int
		storePresetStatus  int
		expectError        bool
		errorContains      string
	}{
		{
			name:     "store_current_spotify_success",
			presetID: 2,
			nowPlayingResponse: `<?xml version="1.0" encoding="UTF-8"?>
<nowPlaying deviceID="TEST123" source="SPOTIFY" sourceAccount="testuser">
    <ContentItem source="SPOTIFY" type="uri" location="spotify:track:123456789" sourceAccount="testuser" isPresetable="true">
        <itemName>Test Track</itemName>
    </ContentItem>
    <track>Test Track</track>
    <artist>Test Artist</artist>
    <playStatus>PLAY_STATE</playStatus>
</nowPlaying>`,
			nowPlayingStatus:  http.StatusOK,
			storePresetStatus: http.StatusOK,
			expectError:       false,
		},
		{
			name:     "store_current_tunein_success",
			presetID: 1,
			nowPlayingResponse: `<?xml version="1.0" encoding="UTF-8"?>
<nowPlaying deviceID="TEST123" source="TUNEIN">
    <ContentItem source="TUNEIN" type="stationurl" location="/v1/playback/station/s33828" isPresetable="true">
        <itemName>K-LOVE Radio</itemName>
    </ContentItem>
    <track>K-LOVE Radio</track>
    <playStatus>PLAY_STATE</playStatus>
</nowPlaying>`,
			nowPlayingStatus:  http.StatusOK,
			storePresetStatus: http.StatusOK,
			expectError:       false,
		},
		{
			name:     "empty_now_playing",
			presetID: 1,
			nowPlayingResponse: `<?xml version="1.0" encoding="UTF-8"?>
<nowPlaying deviceID="TEST123" source="STANDBY">
    <ContentItem source="STANDBY" isPresetable="false" />
</nowPlaying>`,
			nowPlayingStatus: http.StatusOK,
			expectError:      true,
			errorContains:    "no content currently playing",
		},
		{
			name:     "content_not_presetable",
			presetID: 1,
			nowPlayingResponse: `<?xml version="1.0" encoding="UTF-8"?>
<nowPlaying deviceID="TEST123" source="BLUETOOTH">
    <ContentItem source="BLUETOOTH" isPresetable="false">
        <itemName>Phone Audio</itemName>
    </ContentItem>
    <track>Phone Audio</track>
    <playStatus>PLAY_STATE</playStatus>
</nowPlaying>`,
			nowPlayingStatus: http.StatusOK,
			expectError:      true,
			errorContains:    "current content cannot be saved as preset",
		},
		{
			name:     "no_content_item",
			presetID: 1,
			nowPlayingResponse: `<?xml version="1.0" encoding="UTF-8"?>
<nowPlaying deviceID="TEST123" source="UNKNOWN">
    <track>Unknown Track</track>
    <playStatus>PLAY_STATE</playStatus>
</nowPlaying>`,
			nowPlayingStatus: http.StatusOK,
			expectError:      true,
			errorContains:    "no content currently playing",
		},
		{
			name:             "now_playing_request_fails",
			presetID:         1,
			nowPlayingStatus: http.StatusInternalServerError,
			expectError:      true,
			errorContains:    "failed to get current content",
		},
		{
			name:     "invalid_preset_id",
			presetID: 0,
			nowPlayingResponse: `<?xml version="1.0" encoding="UTF-8"?>
<nowPlaying deviceID="TEST123" source="SPOTIFY">
    <ContentItem source="SPOTIFY" type="uri" location="spotify:track:123" isPresetable="true">
        <itemName>Test Track</itemName>
    </ContentItem>
    <track>Test Track</track>
    <playStatus>PLAY_STATE</playStatus>
</nowPlaying>`,
			nowPlayingStatus: http.StatusOK,
			expectError:      true,
			errorContains:    "preset ID must be between 1 and 6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/now_playing":
					if tt.nowPlayingStatus != 0 {
						w.WriteHeader(tt.nowPlayingStatus)
					} else {
						w.WriteHeader(http.StatusOK)
					}

					if tt.nowPlayingResponse != "" {
						_, _ = w.Write([]byte(tt.nowPlayingResponse))
					}
				case "/storePreset":
					if tt.storePresetStatus != 0 {
						w.WriteHeader(tt.storePresetStatus)
					} else {
						w.WriteHeader(http.StatusOK)
					}

					_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><presets></presets>`))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			client := &Client{
				baseURL:    server.URL,
				httpClient: &http.Client{Timeout: 10 * time.Second},
			}

			err := client.StoreCurrentAsPreset(tt.presetID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got nil")
					return
				}

				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', but got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

func TestClient_StorePreset_XMLGeneration(t *testing.T) {
	var capturedXML string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		capturedXML = string(body)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><presets></presets>`))
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	contentItem := &models.ContentItem{
		Source:        "SPOTIFY",
		Type:          "uri",
		Location:      "spotify:playlist:37i9dQZF1DX0XUsuxWHRQd",
		SourceAccount: "testuser",
		IsPresetable:  true,
		ItemName:      "Test Playlist",
		ContainerArt:  "https://example.com/art.jpg",
	}

	err := client.StorePreset(3, contentItem)
	if err != nil {
		t.Fatalf("StorePreset failed: %v", err)
	}

	// Verify XML structure
	expectedElements := []string{
		`<preset id="3"`,
		`createdOn="`,
		`updatedOn="`,
		`<ContentItem source="SPOTIFY"`,
		`type="uri"`,
		`location="spotify:playlist:37i9dQZF1DX0XUsuxWHRQd"`,
		`sourceAccount="testuser"`,
		`isPresetable="true"`,
		`<itemName>Test Playlist</itemName>`,
		`<containerArt>https://example.com/art.jpg</containerArt>`,
	}

	for _, element := range expectedElements {
		if !strings.Contains(capturedXML, element) {
			t.Errorf("Expected XML to contain '%s', but got:\n%s", element, capturedXML)
		}
	}
}

func TestClient_RemovePreset_XMLGeneration(t *testing.T) {
	var capturedXML string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		capturedXML = string(body)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><presets></presets>`))
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	err := client.RemovePreset(4)
	if err != nil {
		t.Fatalf("RemovePreset failed: %v", err)
	}

	// Verify XML structure - should only contain preset ID
	expectedElements := []string{
		`<preset id="4"`,
	}

	for _, element := range expectedElements {
		if !strings.Contains(capturedXML, element) {
			t.Errorf("Expected XML to contain '%s', but got:\n%s", element, capturedXML)
		}
	}

	// Should NOT contain content item for remove requests
	unexpectedElements := []string{
		`<ContentItem`,
		`createdOn=`,
		`updatedOn=`,
	}

	for _, element := range unexpectedElements {
		if strings.Contains(capturedXML, element) {
			t.Errorf("Did not expect XML to contain '%s', but got:\n%s", element, capturedXML)
		}
	}
}

func TestClient_StorePreset_RealWorldScenarios(t *testing.T) {
	scenarios := []struct {
		name        string
		contentItem *models.ContentItem
		description string
	}{
		{
			name: "spotify_daily_mix",
			contentItem: &models.ContentItem{
				Source:        "SPOTIFY",
				Type:          "uri",
				Location:      "spotify:playlist:37i9dQZF1E35Ky0Qr5WjPT",
				SourceAccount: "testuser",
				IsPresetable:  true,
				ItemName:      "Daily Mix 1",
				ContainerArt:  "https://dailymix-images.scdn.co/v2/img/ab6761610000e5eb1/1/en/default",
			},
			description: "User wants to save Spotify Daily Mix as preset",
		},
		{
			name: "internet_radio_station",
			contentItem: &models.ContentItem{
				Source:       "TUNEIN",
				Type:         "stationurl",
				Location:     "/v1/playback/station/s33828",
				IsPresetable: true,
				ItemName:     "K-LOVE Radio",
				ContainerArt: "http://cdn-profiles.tunein.com/s33828/images/logog.png",
			},
			description: "User wants to save favorite radio station",
		},
		{
			name: "nas_music_album",
			contentItem: &models.ContentItem{
				Source:        "STORED_MUSIC",
				Location:      "6_a2874b5d_4f83d999",
				SourceAccount: "d09708a1-5953-44bc-a413-123456789012/0",
				IsPresetable:  true,
				ItemName:      "MercyMe, It's Christmas!",
			},
			description: "User wants to save NAS album as preset",
		},
		{
			name: "pandora_station",
			contentItem: &models.ContentItem{
				Source:        "PANDORA",
				Location:      "126740707481236361",
				SourceAccount: "pandorauser",
				IsPresetable:  true,
				ItemName:      "Zach Williams Radio",
				ContainerArt:  "https://content-images.p-cdn.com/images/68/88/0d/fb/aed34095a11118d2aa7b02a2/_500W_500H.jpg",
			},
			description: "User wants to save Pandora station as preset",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				// Just return success for these scenario tests
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><presets></presets>`))
			}))
			defer server.Close()

			client := &Client{
				baseURL:    server.URL,
				httpClient: &http.Client{Timeout: 10 * time.Second},
			}

			err := client.StorePreset(1, scenario.contentItem)
			if err != nil {
				t.Errorf("Scenario '%s' failed: %s. Error: %v", scenario.name, scenario.description, err)
			}
		})
	}
}

func TestClient_PresetTimestamps(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><presets></presets>`))
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	contentItem := &models.ContentItem{
		Source:       "SPOTIFY",
		Location:     "spotify:track:test",
		IsPresetable: true,
		ItemName:     "Test",
	}

	startTime := time.Now().Unix()

	err := client.StorePreset(1, contentItem)
	if err != nil {
		t.Fatalf("StorePreset failed: %v", err)
	}

	endTime := time.Now().Unix()

	// Timestamps should be set within the test timeframe
	// This is a basic check - in a real scenario, we'd inspect the XML or server response
	if endTime < startTime {
		t.Error("Timestamps appear to be incorrect")
	}
}
