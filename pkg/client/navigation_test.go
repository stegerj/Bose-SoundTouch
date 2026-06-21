package client

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stegerj/bose-soundtouch/pkg/models"
)

// Constants are already defined in other test files

func TestClient_Navigate(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		sourceAccount  string
		startItem      int
		numItems       int
		serverResponse string
		serverStatus   int
		expectError    bool
		errorContains  string
	}{
		{
			name:          "Valid TUNEIN navigate",
			source:        "TUNEIN",
			sourceAccount: "",
			startItem:     1,
			numItems:      50,
			serverResponse: `<?xml version="1.0" encoding="UTF-8"?>
<navigateResponse source="TUNEIN">
	<totalItems>2</totalItems>
	<items>
		<item Playable="1">
			<name>Station 1</name>
			<type>stationurl</type>
			<ContentItem source="TUNEIN" location="/v1/playback/station/s33828" isPresetable="true">
				<itemName>K-LOVE Radio</itemName>
			</ContentItem>
		</item>
		<item Playable="1">
			<name>Station 2</name>
			<type>stationurl</type>
			<ContentItem source="TUNEIN" location="/v1/playback/station/s12345" isPresetable="true">
				<itemName>Test Radio</itemName>
			</ContentItem>
		</item>
	</items>
</navigateResponse>`,
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:          "Valid SPOTIFY navigate with account",
			source:        "SPOTIFY",
			sourceAccount: "user@example.com",
			startItem:     10,
			numItems:      25,
			serverResponse: `<?xml version="1.0" encoding="UTF-8"?>
<navigateResponse source="SPOTIFY" sourceAccount="user@example.com">
	<totalItems>100</totalItems>
	<items>
		<item Playable="1">
			<name>My Playlist</name>
			<type>playlist</type>
			<ContentItem source="SPOTIFY" location="spotify:playlist:123" sourceAccount="user@example.com" isPresetable="true">
				<itemName>My Playlist</itemName>
			</ContentItem>
		</item>
	</items>
</navigateResponse>`,
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:          "Empty source",
			source:        "",
			sourceAccount: "",
			startItem:     1,
			numItems:      50,
			expectError:   true,
			errorContains: "source cannot be empty",
		},
		{
			name:          "Invalid startItem",
			source:        "TUNEIN",
			sourceAccount: "",
			startItem:     0,
			numItems:      50,
			expectError:   true,
			errorContains: "startItem must be >= 1",
		},
		{
			name:          "Invalid numItems",
			source:        "TUNEIN",
			sourceAccount: "",
			startItem:     1,
			numItems:      0,
			expectError:   true,
			errorContains: "numItems must be >= 1",
		},
		{
			name:         "Server error",
			source:       "TUNEIN",
			startItem:    1,
			numItems:     50,
			serverStatus: http.StatusInternalServerError,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.serverStatus != 0 {
					w.WriteHeader(tt.serverStatus)
				}

				if tt.serverResponse != "" {
					_, _ = w.Write([]byte(tt.serverResponse))
				}
			}))
			defer server.Close()

			config := &Config{
				Host:      server.URL[7:],
				Port:      80,
				Timeout:   testTimeout,
				UserAgent: testUserAgent,
			}
			client := NewClient(config)
			client.baseURL = server.URL

			response, err := client.Navigate(tt.source, tt.sourceAccount, tt.startItem, tt.numItems)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}

				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if response == nil {
				t.Error("Expected response but got nil")
				return
			}

			if response.Source != tt.source {
				t.Errorf("Expected source %s, got %s", tt.source, response.Source)
			}
		})
	}
}

func TestClient_NavigateWithMenu(t *testing.T) {
	serverResponse := `<?xml version="1.0" encoding="UTF-8"?>
<navigateResponse source="PANDORA" sourceAccount="user123">
	<totalItems>5</totalItems>
	<items>
		<item Playable="1">
			<name>My Station 1</name>
			<type>stationurl</type>
			<ContentItem source="PANDORA" location="R123456" sourceAccount="user123" isPresetable="true">
				<itemName>My Station 1</itemName>
			</ContentItem>
		</item>
	</items>
</navigateResponse>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request body contains menu and sort parameters
		var request models.NavigateRequest

		err := xml.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if request.Menu != "radioStations" {
			t.Errorf("Expected menu 'radioStations', got %s", request.Menu)
		}

		if request.Sort != "dateCreated" {
			t.Errorf("Expected sort 'dateCreated', got %s", request.Sort)
		}

		_, _ = w.Write([]byte(serverResponse))
	}))
	defer server.Close()

	config := &Config{
		Host:      server.URL[7:],
		Port:      80,
		Timeout:   testTimeout,
		UserAgent: testUserAgent,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	response, err := client.NavigateWithMenu("PANDORA", "user123", "radioStations", "dateCreated", 1, 100)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if response.Source != "PANDORA" {
		t.Errorf("Expected source PANDORA, got %s", response.Source)
	}

	if response.TotalItems != 5 {
		t.Errorf("Expected totalItems 5, got %d", response.TotalItems)
	}
}

func TestClient_NavigateContainer(t *testing.T) {
	containerItem := &models.ContentItem{
		Source:        "STORED_MUSIC",
		Location:      "1",
		SourceAccount: "device123/0",
		IsPresetable:  true,
		ItemName:      "Music",
	}

	serverResponse := `<?xml version="1.0" encoding="UTF-8"?>
<navigateResponse source="STORED_MUSIC" sourceAccount="device123/0">
	<totalItems>3</totalItems>
	<items>
		<item Playable="1">
			<name>Album 1</name>
			<type>dir</type>
			<ContentItem source="STORED_MUSIC" location="album1" sourceAccount="device123/0" isPresetable="true">
				<itemName>Album 1</itemName>
			</ContentItem>
		</item>
	</items>
</navigateResponse>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(serverResponse))
	}))
	defer server.Close()

	config := &Config{
		Host:      server.URL[7:],
		Port:      80,
		Timeout:   testTimeout,
		UserAgent: testUserAgent,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	response, err := client.NavigateContainer("STORED_MUSIC", "device123/0", 1, 1000, containerItem)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if response.Source != "STORED_MUSIC" {
		t.Errorf("Expected source STORED_MUSIC, got %s", response.Source)
	}

	// Test error cases
	_, err = client.NavigateContainer("", "device123/0", 1, 1000, containerItem)
	if err == nil || !contains(err.Error(), "source cannot be empty") {
		t.Error("Expected error for empty source")
	}

	_, err = client.NavigateContainer("STORED_MUSIC", "device123/0", 1, 1000, nil)
	if err == nil || !contains(err.Error(), "container item cannot be nil") {
		t.Error("Expected error for nil container item")
	}
}

func TestClient_AddStation(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		sourceAccount  string
		token          string
		stationName    string
		serverResponse string
		serverStatus   int
		expectError    bool
		errorContains  string
	}{
		{
			name:           "Valid add station",
			source:         "PANDORA",
			sourceAccount:  "user123",
			token:          "R4328162",
			stationName:    "Test Station",
			serverResponse: `<status>/addStation</status>`,
			serverStatus:   http.StatusOK,
			expectError:    false,
		},
		{
			name:          "Empty source",
			source:        "",
			sourceAccount: "user123",
			token:         "R4328162",
			stationName:   "Test Station",
			expectError:   true,
			errorContains: "source cannot be empty",
		},
		{
			name:          "Empty token",
			source:        "PANDORA",
			sourceAccount: "user123",
			token:         "",
			stationName:   "Test Station",
			expectError:   true,
			errorContains: "token cannot be empty",
		},
		{
			name:          "Empty station name",
			source:        "PANDORA",
			sourceAccount: "user123",
			token:         "R4328162",
			stationName:   "",
			expectError:   true,
			errorContains: "station name cannot be empty",
		},
		{
			name:         "Server error",
			source:       "PANDORA",
			token:        "R4328162",
			stationName:  "Test Station",
			serverStatus: http.StatusBadRequest,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.serverStatus != 0 {
					w.WriteHeader(tt.serverStatus)
				}

				if tt.serverResponse != "" {
					_, _ = w.Write([]byte(tt.serverResponse))
				}

				// Verify request format
				if !tt.expectError {
					var request models.AddStationRequest

					err := xml.NewDecoder(r.Body).Decode(&request)
					if err != nil {
						t.Errorf("Failed to decode request: %v", err)
					}

					if request.Source != tt.source {
						t.Errorf("Expected source %s, got %s", tt.source, request.Source)
					}

					if request.Token != tt.token {
						t.Errorf("Expected token %s, got %s", tt.token, request.Token)
					}

					if request.Name != tt.stationName {
						t.Errorf("Expected name %s, got %s", tt.stationName, request.Name)
					}
				}
			}))
			defer server.Close()

			config := &Config{
				Host:      server.URL[7:],
				Port:      80,
				Timeout:   testTimeout,
				UserAgent: testUserAgent,
			}
			client := NewClient(config)
			client.baseURL = server.URL

			err := client.AddStation(tt.source, tt.sourceAccount, tt.token, tt.stationName)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestClient_RemoveStation(t *testing.T) {
	contentItem := &models.ContentItem{
		Source:        "PANDORA",
		Location:      "126740707481236361",
		SourceAccount: "user123",
		IsPresetable:  true,
		ItemName:      "Test Station",
	}

	tests := []struct {
		name           string
		contentItem    *models.ContentItem
		serverResponse string
		serverStatus   int
		expectError    bool
		errorContains  string
	}{
		{
			name:           "Valid remove station",
			contentItem:    contentItem,
			serverResponse: `<status>/removeStation</status>`,
			serverStatus:   http.StatusOK,
			expectError:    false,
		},
		{
			name:          "Nil content item",
			contentItem:   nil,
			expectError:   true,
			errorContains: "content item cannot be nil",
		},
		{
			name: "Empty source",
			contentItem: &models.ContentItem{
				Source:   "",
				Location: "123",
			},
			expectError:   true,
			errorContains: "content item source cannot be empty",
		},
		{
			name: "Empty location",
			contentItem: &models.ContentItem{
				Source:   "PANDORA",
				Location: "",
			},
			expectError:   true,
			errorContains: "content item location cannot be empty",
		},
		{
			name:         "Server error",
			contentItem:  contentItem,
			serverStatus: http.StatusNotFound,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.serverStatus != 0 {
					w.WriteHeader(tt.serverStatus)
				}

				if tt.serverResponse != "" {
					_, _ = w.Write([]byte(tt.serverResponse))
				}

				// Verify request format
				if !tt.expectError && tt.contentItem != nil {
					var request models.ContentItem

					err := xml.NewDecoder(r.Body).Decode(&request)
					if err != nil {
						t.Errorf("Failed to decode request: %v", err)
					}

					if request.Source != tt.contentItem.Source {
						t.Errorf("Expected source %s, got %s", tt.contentItem.Source, request.Source)
					}

					if request.Location != tt.contentItem.Location {
						t.Errorf("Expected location %s, got %s", tt.contentItem.Location, request.Location)
					}
				}
			}))
			defer server.Close()

			config := &Config{
				Host:      server.URL[7:],
				Port:      80,
				Timeout:   testTimeout,
				UserAgent: testUserAgent,
			}
			client := NewClient(config)
			client.baseURL = server.URL

			err := client.RemoveStation(tt.contentItem)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestClient_GetPandoraStations(t *testing.T) {
	serverResponse := `<?xml version="1.0" encoding="UTF-8"?>
<navigateResponse source="PANDORA" sourceAccount="user123">
	<totalItems>2</totalItems>
	<items>
		<item Playable="1">
			<name>Station 1</name>
			<type>stationurl</type>
			<ContentItem source="PANDORA" location="R123" sourceAccount="user123" isPresetable="true">
				<itemName>Station 1</itemName>
			</ContentItem>
		</item>
	</items>
</navigateResponse>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's calling navigate with the right parameters
		var request models.NavigateRequest

		_ = xml.NewDecoder(r.Body).Decode(&request)

		if request.Source != "PANDORA" {
			t.Errorf("Expected source PANDORA, got %s", request.Source)
		}

		if request.Menu != "radioStations" {
			t.Errorf("Expected menu radioStations, got %s", request.Menu)
		}

		if request.Sort != "dateCreated" {
			t.Errorf("Expected sort dateCreated, got %s", request.Sort)
		}

		_, _ = w.Write([]byte(serverResponse))
	}))
	defer server.Close()

	config := &Config{
		Host:      server.URL[7:],
		Port:      80,
		Timeout:   testTimeout,
		UserAgent: testUserAgent,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	response, err := client.GetPandoraStations("user123")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if response.Source != "PANDORA" {
		t.Errorf("Expected source PANDORA, got %s", response.Source)
	}

	// Test error case
	_, err = client.GetPandoraStations("")
	if err == nil || !contains(err.Error(), "pandora source account cannot be empty") {
		t.Error("Expected error for empty source account")
	}
}

func TestClient_GetTuneInStations(t *testing.T) {
	serverResponse := `<?xml version="1.0" encoding="UTF-8"?>
<navigateResponse source="TUNEIN">
	<totalItems>1</totalItems>
	<items>
		<item Playable="1">
			<name>Radio Station</name>
			<type>stationurl</type>
			<ContentItem source="TUNEIN" location="/v1/playback/station/s12345" isPresetable="true">
				<itemName>Radio Station</itemName>
			</ContentItem>
		</item>
	</items>
</navigateResponse>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(serverResponse))
	}))
	defer server.Close()

	config := &Config{
		Host:      server.URL[7:],
		Port:      80,
		Timeout:   testTimeout,
		UserAgent: testUserAgent,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	response, err := client.GetTuneInStations("Rock")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if response.Source != "TUNEIN" {
		t.Errorf("Expected source TUNEIN, got %s", response.Source)
	}
}

func TestClient_GetStoredMusicLibrary(t *testing.T) {
	serverResponse := `<?xml version="1.0" encoding="UTF-8"?>
<navigateResponse source="STORED_MUSIC" sourceAccount="device123/0">
	<totalItems>1</totalItems>
	<items>
		<item Playable="1">
			<name>My Music</name>
			<type>dir</type>
			<ContentItem source="STORED_MUSIC" location="1" sourceAccount="device123/0" isPresetable="true">
				<itemName>My Music</itemName>
			</ContentItem>
		</item>
	</items>
</navigateResponse>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(serverResponse))
	}))
	defer server.Close()

	config := &Config{
		Host:      server.URL[7:],
		Port:      80,
		Timeout:   testTimeout,
		UserAgent: testUserAgent,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	response, err := client.GetStoredMusicLibrary("device123/0")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if response.Source != "STORED_MUSIC" {
		t.Errorf("Expected source STORED_MUSIC, got %s", response.Source)
	}

	// Test error case
	_, err = client.GetStoredMusicLibrary("")
	if err == nil || !contains(err.Error(), "stored music source account cannot be empty") {
		t.Error("Expected error for empty source account")
	}
}

func TestClient_SearchStation(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		sourceAccount  string
		searchTerm     string
		serverResponse string
		serverStatus   int
		expectError    bool
		errorContains  string
	}{
		{
			name:          "Valid Pandora search",
			source:        "PANDORA",
			sourceAccount: "user123",
			searchTerm:    "Zach Williams",
			serverResponse: `<?xml version="1.0" encoding="UTF-8"?>
<results deviceID="1004567890AA" source="PANDORA" sourceAccount="user123">
	<songs>
		<searchResult source="PANDORA" sourceAccount="user123" token="S10657777">
			<name>Old Church Choir</name>
			<artist>Zach Williams</artist>
			<logo>http://example.com/song.jpg</logo>
		</searchResult>
	</songs>
	<artists>
		<searchResult source="PANDORA" sourceAccount="user123" token="R324771">
			<name>Zach Williams</name>
			<logo>http://example.com/artist.jpg</logo>
		</searchResult>
	</artists>
</results>`,
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:          "Valid TuneIn search",
			source:        "TUNEIN",
			sourceAccount: "",
			searchTerm:    "Classic Rock",
			serverResponse: `<?xml version="1.0" encoding="UTF-8"?>
<results deviceID="1004567890AA" source="TUNEIN">
	<stations>
		<searchResult source="TUNEIN" token="s12345">
			<name>Classic Rock 101.5</name>
			<description>The best classic rock hits</description>
			<logo>http://example.com/station.jpg</logo>
		</searchResult>
	</stations>
</results>`,
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:          "Empty source",
			source:        "",
			sourceAccount: "user123",
			searchTerm:    "test",
			expectError:   true,
			errorContains: "source cannot be empty",
		},
		{
			name:          "Empty search term",
			source:        "PANDORA",
			sourceAccount: "user123",
			searchTerm:    "",
			expectError:   true,
			errorContains: "search term cannot be empty",
		},
		{
			name:          "Server error",
			source:        "PANDORA",
			sourceAccount: "user123",
			searchTerm:    "test",
			serverStatus:  http.StatusBadRequest,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.serverStatus != 0 {
					w.WriteHeader(tt.serverStatus)
				}

				if tt.serverResponse != "" {
					_, _ = w.Write([]byte(tt.serverResponse))
				}

				// Verify request format for valid requests
				if !tt.expectError {
					var request models.SearchStationRequest

					err := xml.NewDecoder(r.Body).Decode(&request)
					if err != nil {
						t.Errorf("Failed to decode request: %v", err)
					}

					if request.Source != tt.source {
						t.Errorf("Expected source %s, got %s", tt.source, request.Source)
					}

					if request.SearchTerm != tt.searchTerm {
						t.Errorf("Expected searchTerm %s, got %s", tt.searchTerm, request.SearchTerm)
					}
				}
			}))
			defer server.Close()

			config := &Config{
				Host:      server.URL[7:],
				Port:      80,
				Timeout:   testTimeout,
				UserAgent: testUserAgent,
			}
			client := NewClient(config)
			client.baseURL = server.URL
			response, err := client.SearchStation(tt.source, tt.sourceAccount, tt.searchTerm)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}

				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if response == nil {
				t.Error("Expected response but got nil")
				return
			}

			if response.Source != tt.source {
				t.Errorf("Expected source %s, got %s", tt.source, response.Source)
			}
		})
	}
}

func TestClient_SearchPandoraStations(t *testing.T) {
	serverResponse := `<?xml version="1.0" encoding="UTF-8"?>
<results deviceID="1004567890AA" source="PANDORA" sourceAccount="user123">
	<artists>
		<searchResult source="PANDORA" sourceAccount="user123" token="R324771">
			<name>Taylor Swift</name>
			<logo>http://example.com/artist.jpg</logo>
		</searchResult>
	</artists>
</results>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's calling searchStation with the right parameters
		var request models.SearchStationRequest

		_ = xml.NewDecoder(r.Body).Decode(&request)

		if request.Source != "PANDORA" {
			t.Errorf("Expected source PANDORA, got %s", request.Source)
		}

		if request.SourceAccount != "user123" {
			t.Errorf("Expected sourceAccount user123, got %s", request.SourceAccount)
		}

		if request.SearchTerm != "Taylor Swift" {
			t.Errorf("Expected searchTerm 'Taylor Swift', got %s", request.SearchTerm)
		}

		_, _ = w.Write([]byte(serverResponse))
	}))
	defer server.Close()

	config := &Config{
		Host:      server.URL[7:],
		Port:      80,
		Timeout:   testTimeout,
		UserAgent: testUserAgent,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	response, err := client.SearchPandoraStations("user123", "Taylor Swift")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if response.Source != "PANDORA" {
		t.Errorf("Expected source PANDORA, got %s", response.Source)
	}

	// Test error case
	_, err = client.SearchPandoraStations("", "test")
	if err == nil || !contains(err.Error(), "pandora source account cannot be empty") {
		t.Error("Expected error for empty source account")
	}
}

func TestClient_SearchTuneInStations(t *testing.T) {
	serverResponse := `<?xml version="1.0" encoding="UTF-8"?>
<results deviceID="1004567890AA" source="TUNEIN">
	<stations>
		<searchResult source="TUNEIN" token="s12345">
			<name>Jazz 24/7</name>
			<description>Smooth jazz all day</description>
			<logo>http://example.com/jazz.jpg</logo>
		</searchResult>
	</stations>
</results>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(serverResponse))
	}))
	defer server.Close()

	config := &Config{
		Host:      server.URL[7:],
		Port:      80,
		Timeout:   testTimeout,
		UserAgent: testUserAgent,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	response, err := client.SearchTuneInStations("Jazz")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if response.Source != "TUNEIN" {
		t.Errorf("Expected source TUNEIN, got %s", response.Source)
	}

	if len(response.Stations) != 1 {
		t.Errorf("Expected 1 station result, got %d", len(response.Stations))
	}
}

func TestClient_SearchSpotifyContent(t *testing.T) {
	serverResponse := `<?xml version="1.0" encoding="UTF-8"?>
<results deviceID="1004567890AA" source="SPOTIFY" sourceAccount="user@example.com">
	<songs>
		<searchResult source="SPOTIFY" sourceAccount="user@example.com" token="track123">
			<name>Bohemian Rhapsody</name>
			<artist>Queen</artist>
			<album>A Night at the Opera</album>
			<logo>http://example.com/queen.jpg</logo>
		</searchResult>
	</songs>
</results>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(serverResponse))
	}))
	defer server.Close()

	config := &Config{
		Host:      server.URL[7:],
		Port:      80,
		Timeout:   testTimeout,
		UserAgent: testUserAgent,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	response, err := client.SearchSpotifyContent("user@example.com", "Queen")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if response.Source != "SPOTIFY" {
		t.Errorf("Expected source SPOTIFY, got %s", response.Source)
	}

	// Test error case
	_, err = client.SearchSpotifyContent("", "test")
	if err == nil || !contains(err.Error(), "spotify source account cannot be empty") {
		t.Error("Expected error for empty source account")
	}
}

// Helper functions are already defined in other test files
