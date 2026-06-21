package client

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stegerj/bose-soundtouch/pkg/models"
)

func TestClient_GetRecents(t *testing.T) {
	tests := []struct {
		name          string
		responseXML   string
		statusCode    int
		expectedError string
		wantResponse  *models.RecentsResponse
	}{
		{
			name:       "successful recents response",
			statusCode: http.StatusOK,
			responseXML: `<?xml version="1.0" encoding="UTF-8" ?>
<recents>
  <recent deviceID="1004567890AA" utcTime="1701202831">
    <contentItem source="STORED_MUSIC" location="6_a2874b5d_4f83d999" sourceAccount="d09708a1-5953-44bc-a413-123456789012/0" isPresetable="true">
      <itemName>MercyMe, It's Christmas!</itemName>
    </contentItem>
  </recent>
  <recent deviceID="1004567890AA" utcTime="1700232917" id="2487503626">
    <contentItem source="LOCAL_MUSIC" type="track" location="track:2590" sourceAccount="3f205110-4a57-4e91-810a-123456789012" isPresetable="true">
      <itemName>Baby It's Cold Outside - ANNE MURRAY</itemName>
    </contentItem>
  </recent>
</recents>`,
			wantResponse: &models.RecentsResponse{
				Items: []models.RecentsResponseItem{
					{
						DeviceID: "1004567890AA",
						UTCTime:  1701202831,
						ContentItem: &models.ContentItem{
							Source:        "STORED_MUSIC",
							Location:      "6_a2874b5d_4f83d999",
							SourceAccount: "d09708a1-5953-44bc-a413-123456789012/0",
							IsPresetable:  true,
							ItemName:      "MercyMe, It's Christmas!",
						},
					},
					{
						DeviceID: "1004567890AA",
						UTCTime:  1700232917,
						ID:       "2487503626",
						ContentItem: &models.ContentItem{
							Source:        "LOCAL_MUSIC",
							Type:          "track",
							Location:      "track:2590",
							SourceAccount: "3f205110-4a57-4e91-810a-123456789012",
							IsPresetable:  true,
							ItemName:      "Baby It's Cold Outside - ANNE MURRAY",
						},
					},
				},
			},
		},
		{
			name:       "empty recents response",
			statusCode: http.StatusOK,
			responseXML: `<?xml version="1.0" encoding="UTF-8" ?>
<recents>
</recents>`,
			wantResponse: &models.RecentsResponse{
				Items: []models.RecentsResponseItem{},
			},
		},
		{
			name:       "spotify recents with artwork",
			statusCode: http.StatusOK,
			responseXML: `<?xml version="1.0" encoding="UTF-8" ?>
<recents>
  <recent deviceID="1004567890AA" utcTime="1701300000" id="spotify123">
    <contentItem source="SPOTIFY" type="track" location="spotify:track:4iV5W9uYEdYUVa79Axb7Rh" sourceAccount="spotify_user" isPresetable="true">
      <itemName>Shape of You - Ed Sheeran</itemName>
      <containerArt>https://i.scdn.co/image/ab67616d0000b273ba5db46f4b838ef6027e6f96</containerArt>
    </contentItem>
  </recent>
  <recent deviceID="1004567890AA" utcTime="1701250000" id="spotify124">
    <contentItem source="SPOTIFY" type="playlist" location="spotify:playlist:37i9dQZF1DXcBWIGoYBM5M" sourceAccount="spotify_user" isPresetable="true">
      <itemName>Today's Top Hits</itemName>
      <containerArt>https://i.scdn.co/image/ab67706f00000002ca5a7517156021292e5663a6</containerArt>
    </contentItem>
  </recent>
</recents>`,
			wantResponse: &models.RecentsResponse{
				Items: []models.RecentsResponseItem{
					{
						DeviceID: "1004567890AA",
						UTCTime:  1701300000,
						ID:       "spotify123",
						ContentItem: &models.ContentItem{
							Source:        "SPOTIFY",
							Type:          "track",
							Location:      "spotify:track:4iV5W9uYEdYUVa79Axb7Rh",
							SourceAccount: "spotify_user",
							IsPresetable:  true,
							ItemName:      "Shape of You - Ed Sheeran",
							ContainerArt:  "https://i.scdn.co/image/ab67616d0000b273ba5db46f4b838ef6027e6f96",
						},
					},
					{
						DeviceID: "1004567890AA",
						UTCTime:  1701250000,
						ID:       "spotify124",
						ContentItem: &models.ContentItem{
							Source:        "SPOTIFY",
							Type:          "playlist",
							Location:      "spotify:playlist:37i9dQZF1DXcBWIGoYBM5M",
							SourceAccount: "spotify_user",
							IsPresetable:  true,
							ItemName:      "Today's Top Hits",
							ContainerArt:  "https://i.scdn.co/image/ab67706f00000002ca5a7517156021292e5663a6",
						},
					},
				},
			},
		},
		{
			name:       "tunein radio station",
			statusCode: http.StatusOK,
			responseXML: `<?xml version="1.0" encoding="UTF-8" ?>
<recents>
  <recent deviceID="1004567890AA" utcTime="1701400000">
    <contentItem source="TUNEIN" type="stationurl" location="tunein:station:s24939" sourceAccount="tunein" isPresetable="true">
      <itemName>BBC Radio 1</itemName>
    </contentItem>
  </recent>
</recents>`,
			wantResponse: &models.RecentsResponse{
				Items: []models.RecentsResponseItem{
					{
						DeviceID: "1004567890AA",
						UTCTime:  1701400000,
						ContentItem: &models.ContentItem{
							Source:        "TUNEIN",
							Type:          "stationurl",
							Location:      "tunein:station:s24939",
							SourceAccount: "tunein",
							IsPresetable:  true,
							ItemName:      "BBC Radio 1",
						},
					},
				},
			},
		},
		{
			name:          "http error",
			statusCode:    http.StatusInternalServerError,
			responseXML:   "",
			expectedError: "failed to get recent items:",
		},
		{
			name:          "malformed xml",
			statusCode:    http.StatusOK,
			responseXML:   `<invalid>xml</malformed>`,
			expectedError: "failed to get recent items:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				if r.Method != "GET" {
					t.Errorf("expected GET request, got %s", r.Method)
				}

				if r.URL.Path != "/recents" {
					t.Errorf("expected /recents path, got %s", r.URL.Path)
				}

				if tt.statusCode != http.StatusOK {
					w.WriteHeader(tt.statusCode)
					return
				}

				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.responseXML))
			}))
			defer server.Close()

			config := &Config{
				Host: server.URL[7:], // Remove "http://" prefix
				Port: 80,
			}
			client := NewClient(config)
			// Override the base URL to use test server
			client.baseURL = server.URL

			response, err := client.GetRecents()

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectedError)
					return
				}

				if !containsString(err.Error(), tt.expectedError) {
					t.Errorf("expected error containing %q, got %q", tt.expectedError, err.Error())
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if response == nil {
				t.Error("expected response, got nil")
				return
			}

			// Verify response structure
			if len(response.Items) != len(tt.wantResponse.Items) {
				t.Errorf("expected %d items, got %d", len(tt.wantResponse.Items), len(response.Items))
			}

			// Verify each item
			for i, expectedItem := range tt.wantResponse.Items {
				if i >= len(response.Items) {
					break
				}

				actualItem := response.Items[i]

				if actualItem.DeviceID != expectedItem.DeviceID {
					t.Errorf("item %d: expected deviceID %s, got %s", i, expectedItem.DeviceID, actualItem.DeviceID)
				}

				if actualItem.UTCTime != expectedItem.UTCTime {
					t.Errorf("item %d: expected utcTime %d, got %d", i, expectedItem.UTCTime, actualItem.UTCTime)
				}

				if actualItem.ID != expectedItem.ID {
					t.Errorf("item %d: expected id %s, got %s", i, expectedItem.ID, actualItem.ID)
				}

				// Verify ContentItem
				if expectedItem.ContentItem != nil {
					if actualItem.ContentItem == nil {
						t.Errorf("item %d: expected contentItem, got nil", i)
						continue
					}

					if actualItem.ContentItem.Source != expectedItem.ContentItem.Source {
						t.Errorf("item %d: expected source %s, got %s", i, expectedItem.ContentItem.Source, actualItem.ContentItem.Source)
					}

					if actualItem.ContentItem.Type != expectedItem.ContentItem.Type {
						t.Errorf("item %d: expected type %s, got %s", i, expectedItem.ContentItem.Type, actualItem.ContentItem.Type)
					}

					if actualItem.ContentItem.Location != expectedItem.ContentItem.Location {
						t.Errorf("item %d: expected location %s, got %s", i, expectedItem.ContentItem.Location, actualItem.ContentItem.Location)
					}

					if actualItem.ContentItem.ItemName != expectedItem.ContentItem.ItemName {
						t.Errorf("item %d: expected itemName %s, got %s", i, expectedItem.ContentItem.ItemName, actualItem.ContentItem.ItemName)
					}

					if actualItem.ContentItem.IsPresetable != expectedItem.ContentItem.IsPresetable {
						t.Errorf("item %d: expected isPresetable %t, got %t", i, expectedItem.ContentItem.IsPresetable, actualItem.ContentItem.IsPresetable)
					}

					if actualItem.ContentItem.ContainerArt != expectedItem.ContentItem.ContainerArt {
						t.Errorf("item %d: expected containerArt %s, got %s", i, expectedItem.ContentItem.ContainerArt, actualItem.ContentItem.ContainerArt)
					}
				} else if actualItem.ContentItem != nil {
					t.Errorf("item %d: expected nil contentItem, got non-nil", i)
				}
			}
		})
	}
}

func TestRecentsResponse_MethodsIntegration(t *testing.T) {
	// Test the response methods with a realistic response
	xmlData := `<recents>
  <recent deviceID="1004567890AA" utcTime="1701300000" id="1">
    <contentItem source="SPOTIFY" type="track" location="spotify:track:123" isPresetable="true">
      <itemName>Spotify Track</itemName>
    </contentItem>
  </recent>
  <recent deviceID="1004567890AA" utcTime="1701200000" id="2">
    <contentItem source="LOCAL_MUSIC" type="track" location="/music/local.mp3" isPresetable="false">
      <itemName>Local Track</itemName>
    </contentItem>
  </recent>
  <recent deviceID="1004567890AA" utcTime="1701100000" id="3">
    <contentItem source="TUNEIN" type="stationurl" location="tunein:station:123" isPresetable="true">
      <itemName>Radio Station</itemName>
    </contentItem>
  </recent>
  <recent deviceID="1004567890AA" utcTime="1701000000" id="4">
    <contentItem source="PANDORA" type="track" location="pandora:track:456" isPresetable="true">
      <itemName>Pandora Track</itemName>
    </contentItem>
  </recent>
</recents>`

	var response models.RecentsResponse

	err := xml.Unmarshal([]byte(xmlData), &response)
	if err != nil {
		t.Fatalf("failed to unmarshal test data: %v", err)
	}

	// Test various filtering methods
	tests := []struct {
		name     string
		method   func() interface{}
		expected interface{}
	}{
		{"GetItemCount", func() interface{} { return response.GetItemCount() }, 4},
		{"IsEmpty", func() interface{} { return response.IsEmpty() }, false},
		{"GetSpotifyItems count", func() interface{} { return len(response.GetSpotifyItems()) }, 1},
		{"GetLocalMusicItems count", func() interface{} { return len(response.GetLocalMusicItems()) }, 1},
		{"GetTuneInItems count", func() interface{} { return len(response.GetTuneInItems()) }, 1},
		{"GetPandoraItems count", func() interface{} { return len(response.GetPandoraItems()) }, 1},
		{"GetTracks count", func() interface{} { return len(response.GetTracks()) }, 3},
		{"GetStations count", func() interface{} { return len(response.GetStations()) }, 1},
		{"GetPresetableItems count", func() interface{} { return len(response.GetPresetableItems()) }, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}

	// Test most recent item
	mostRecent := response.GetMostRecent()
	if mostRecent == nil {
		t.Error("expected most recent item, got nil")
	} else {
		if mostRecent.GetDisplayName() != "Spotify Track" {
			t.Errorf("expected most recent to be 'Spotify Track', got %s", mostRecent.GetDisplayName())
		}

		if mostRecent.GetUTCTime() != 1701300000 {
			t.Errorf("expected most recent UTC time 1701300000, got %d", mostRecent.GetUTCTime())
		}
	}

	// Test individual item methods
	for i, item := range response.Items {
		t.Run(t.Name()+"/item_"+item.GetID(), func(t *testing.T) {
			if !item.HasContent() {
				t.Error("expected item to have content")
			}

			if item.GetDisplayName() == "" {
				t.Error("expected item to have display name")
			}

			if item.GetSource() == "" {
				t.Error("expected item to have source")
			}

			if item.GetUTCTime() == 0 {
				t.Error("expected item to have UTC time")
			}

			// Test specific item properties
			switch i {
			case 0: // Spotify track
				if !item.IsSpotifyContent() {
					t.Error("expected first item to be Spotify content")
				}

				if !item.IsTrack() {
					t.Error("expected first item to be a track")
				}

				if !item.IsStreamingContent() {
					t.Error("expected first item to be streaming content")
				}
			case 1: // Local music
				if !item.IsLocalContent() {
					t.Error("expected second item to be local content")
				}

				if item.IsStreamingContent() {
					t.Error("expected second item to not be streaming content")
				}
			case 2: // TuneIn station
				if !item.IsStation() {
					t.Error("expected third item to be a station")
				}

				if item.IsTrack() {
					t.Error("expected third item to not be a track")
				}
			case 3: // Pandora track
				if !item.IsStreamingContent() {
					t.Error("expected fourth item to be streaming content")
				}
			}
		})
	}
}
