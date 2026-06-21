package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stegerj/bose-soundtouch/pkg/models"
)

func TestClient_NavigateXMLValidation(t *testing.T) {
	tests := []struct {
		name             string
		source           string
		sourceAccount    string
		startItem        int
		numItems         int
		expectedXML      string
		expectedEndpoint string
	}{
		{
			name:             "Basic navigate XML structure",
			source:           "TUNEIN",
			sourceAccount:    "",
			startItem:        1,
			numItems:         25,
			expectedXML:      `<navigate source="TUNEIN"><startItem>1</startItem><numItems>25</numItems></navigate>`,
			expectedEndpoint: "/navigate",
		},
		{
			name:             "Navigate with source account",
			source:           "SPOTIFY",
			sourceAccount:    "user@example.com",
			startItem:        10,
			numItems:         50,
			expectedXML:      `<navigate source="SPOTIFY" sourceAccount="user@example.com"><startItem>10</startItem><numItems>50</numItems></navigate>`,
			expectedEndpoint: "/navigate",
		},
		{
			name:             "Navigate stored music with device account",
			source:           "STORED_MUSIC",
			sourceAccount:    "device123456/0",
			startItem:        1,
			numItems:         1000,
			expectedXML:      `<navigate source="STORED_MUSIC" sourceAccount="device123456/0"><startItem>1</startItem><numItems>1000</numItems></navigate>`,
			expectedEndpoint: "/navigate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				capturedXML      string
				capturedEndpoint string
			)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedEndpoint = r.URL.Path

				body := make([]byte, r.ContentLength)
				_, _ = r.Body.Read(body)
				capturedXML = string(body)

				// Return valid navigate response
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<navigateResponse source="` + tt.source + `">
	<totalItems>0</totalItems>
	<items></items>
</navigateResponse>`))
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

			_, err := client.Navigate(tt.source, tt.sourceAccount, tt.startItem, tt.numItems)
			if err != nil {
				t.Fatalf("Navigate failed: %v", err)
			}

			if capturedEndpoint != tt.expectedEndpoint {
				t.Errorf("Expected endpoint %s, got %s", tt.expectedEndpoint, capturedEndpoint)
			}

			if capturedXML != tt.expectedXML {
				t.Errorf("XML mismatch:\nExpected: %s\nActual:   %s", tt.expectedXML, capturedXML)
			}
		})
	}
}

func TestClient_NavigateWithMenuXMLValidation(t *testing.T) {
	var capturedXML string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		capturedXML = string(body)

		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<navigateResponse source="SPOTIFY">
	<totalItems>0</totalItems>
	<items></items>
</navigateResponse>`))
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

	_, err := client.NavigateWithMenu("PANDORA", "user123", "radioStations", "dateCreated", 1, 100)
	if err != nil {
		t.Fatalf("NavigateWithMenu failed: %v", err)
	}

	expectedXML := `<navigate source="PANDORA" sourceAccount="user123" menu="radioStations" sort="dateCreated"><startItem>1</startItem><numItems>100</numItems></navigate>`
	if capturedXML != expectedXML {
		t.Errorf("XML mismatch:\nExpected: %s\nActual:   %s", expectedXML, capturedXML)
	}
}

func TestClient_SearchStationXMLValidation(t *testing.T) {
	tests := []struct {
		name          string
		source        string
		sourceAccount string
		searchTerm    string
		expectedXML   string
	}{
		{
			name:          "Basic search XML",
			source:        "PANDORA",
			sourceAccount: "user123",
			searchTerm:    "Taylor Swift",
			expectedXML:   `<search source="PANDORA" sourceAccount="user123">Taylor Swift</search>`,
		},
		{
			name:          "Search without account",
			source:        "TUNEIN",
			sourceAccount: "",
			searchTerm:    "Jazz Radio",
			expectedXML:   `<search source="TUNEIN">Jazz Radio</search>`,
		},
		{
			name:          "Search with special characters",
			source:        "SPOTIFY",
			sourceAccount: "user@example.com",
			searchTerm:    "Rock & Roll",
			expectedXML:   `<search source="SPOTIFY" sourceAccount="user@example.com">Rock &amp; Roll</search>`,
		},
		{
			name:          "Search with quotes",
			source:        "PANDORA",
			sourceAccount: "user",
			searchTerm:    `"The Beatles"`,
			expectedXML:   `<search source="PANDORA" sourceAccount="user">&#34;The Beatles&#34;</search>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedXML string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body := make([]byte, r.ContentLength)
				_, _ = r.Body.Read(body)
				capturedXML = string(body)

				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<results source="` + tt.source + `">
	<songs></songs>
	<artists></artists>
	<stations></stations>
</results>`))
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

			_, err := client.SearchStation(tt.source, tt.sourceAccount, tt.searchTerm)
			if err != nil {
				t.Fatalf("SearchStation failed: %v", err)
			}

			if capturedXML != tt.expectedXML {
				t.Errorf("XML mismatch:\nExpected: %s\nActual:   %s", tt.expectedXML, capturedXML)
			}
		})
	}
}

func TestClient_AddStationXMLValidation(t *testing.T) {
	var capturedXML string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		capturedXML = string(body)

		_, _ = w.Write([]byte(`<status>/addStation</status>`))
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

	err := client.AddStation("PANDORA", "user123", "R4328162", "Test Station")
	if err != nil {
		t.Fatalf("AddStation failed: %v", err)
	}

	expectedXML := `<addStation source="PANDORA" sourceAccount="user123" token="R4328162"><name>Test Station</name></addStation>`
	if capturedXML != expectedXML {
		t.Errorf("XML mismatch:\nExpected: %s\nActual:   %s", expectedXML, capturedXML)
	}
}

func TestClient_RemoveStationXMLValidation(t *testing.T) {
	contentItem := &models.ContentItem{
		Source:        "PANDORA",
		Location:      "126740707481236361",
		SourceAccount: "user123",
		IsPresetable:  true,
		ItemName:      "Test Station",
	}

	var capturedXML string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		capturedXML = string(body)

		_, _ = w.Write([]byte(`<status>/removeStation</status>`))
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

	err := client.RemoveStation(contentItem)
	if err != nil {
		t.Fatalf("RemoveStation failed: %v", err)
	}

	// Verify the XML contains the expected ContentItem structure
	if !strings.Contains(capturedXML, `source="PANDORA"`) {
		t.Error("XML should contain source attribute")
	}

	if !strings.Contains(capturedXML, `location="126740707481236361"`) {
		t.Error("XML should contain location attribute")
	}

	if !strings.Contains(capturedXML, `<itemName>Test Station</itemName>`) {
		t.Error("XML should contain itemName element")
	}
}

func TestClient_NavigationResponseParsing(t *testing.T) {
	tests := []struct {
		name          string
		responseXML   string
		expectError   bool
		expectedItems int
		expectedTotal int
	}{
		{
			name: "Valid complex response",
			responseXML: `<?xml version="1.0" encoding="UTF-8"?>
<navigateResponse source="STORED_MUSIC" sourceAccount="device123/0">
	<totalItems>3</totalItems>
	<items>
		<item Playable="1">
			<name>Album Artists</name>
			<type>dir</type>
			<ContentItem source="STORED_MUSIC" location="107" sourceAccount="device123/0" isPresetable="true">
				<itemName>Album Artists</itemName>
				<containerArt>http://example.com/art.jpg</containerArt>
			</ContentItem>
		</item>
		<item Playable="1">
			<name>Test Track</name>
			<type>track</type>
			<ContentItem source="STORED_MUSIC" location="track123" sourceAccount="device123/0" isPresetable="true">
				<itemName>Test Track</itemName>
			</ContentItem>
			<artistName>Test Artist</artistName>
			<albumName>Test Album</albumName>
		</item>
		<item Playable="0">
			<name>Non-playable Item</name>
			<type>unknown</type>
		</item>
	</items>
</navigateResponse>`,
			expectError:   false,
			expectedItems: 3,
			expectedTotal: 3,
		},
		{
			name: "Empty response",
			responseXML: `<?xml version="1.0" encoding="UTF-8"?>
<navigateResponse source="TUNEIN">
	<totalItems>0</totalItems>
	<items></items>
</navigateResponse>`,
			expectError:   false,
			expectedItems: 0,
			expectedTotal: 0,
		},
		{
			name: "Invalid XML",
			responseXML: `<?xml version="1.0" encoding="UTF-8"?>
<navigateResponse source="TUNEIN">
	<totalItems>1</totalItems>
	<items>
		<item>
			<name>Unclosed item
		</item>
	</items>`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.responseXML))
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

			response, err := client.Navigate("TUNEIN", "", 1, 10)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}

				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(response.Items) != tt.expectedItems {
				t.Errorf("Expected %d items, got %d", tt.expectedItems, len(response.Items))
			}

			if response.TotalItems != tt.expectedTotal {
				t.Errorf("Expected total %d, got %d", tt.expectedTotal, response.TotalItems)
			}

			// Test helper methods for complex response
			if tt.name == "Valid complex response" {
				playable := response.GetPlayableItems()
				if len(playable) != 2 {
					t.Errorf("Expected 2 playable items, got %d", len(playable))
				}

				directories := response.GetDirectories()
				if len(directories) != 1 {
					t.Errorf("Expected 1 directory, got %d", len(directories))
				}

				tracks := response.GetTracks()
				if len(tracks) != 1 {
					t.Errorf("Expected 1 track, got %d", len(tracks))
				}

				// Test individual item properties
				firstItem := response.Items[0]
				if !firstItem.IsPlayable() {
					t.Error("First item should be playable")
				}

				if !firstItem.IsDirectory() {
					t.Error("First item should be directory")
				}

				if firstItem.GetArtwork() == "" {
					t.Error("First item should have artwork")
				}

				secondItem := response.Items[1]
				if !secondItem.IsTrack() {
					t.Error("Second item should be track")
				}

				if secondItem.ArtistName != "Test Artist" {
					t.Errorf("Expected artist 'Test Artist', got %s", secondItem.ArtistName)
				}

				thirdItem := response.Items[2]
				if thirdItem.IsPlayable() {
					t.Error("Third item should not be playable")
				}
			}
		})
	}
}

func TestClient_SearchStationResponseParsing(t *testing.T) {
	responseXML := `<?xml version="1.0" encoding="UTF-8"?>
<results deviceID="1004567890AA" source="PANDORA" sourceAccount="user123">
	<songs>
		<searchResult source="PANDORA" sourceAccount="user123" token="S10657777">
			<name>Old Church Choir</name>
			<artist>Zach Williams</artist>
			<album>Chain Breaker</album>
			<logo>http://example.com/song.jpg</logo>
		</searchResult>
		<searchResult source="PANDORA" sourceAccount="user123" token="S10657778">
			<name>Fear Is a Liar</name>
			<artist>Zach Williams</artist>
			<logo>http://example.com/song2.jpg</logo>
		</searchResult>
	</songs>
	<artists>
		<searchResult source="PANDORA" sourceAccount="user123" token="R324771">
			<name>Zach Williams</name>
			<logo>http://example.com/artist.jpg</logo>
		</searchResult>
	</artists>
	<stations>
		<searchResult source="PANDORA" sourceAccount="user123" token="R123456">
			<name>Christian Rock Radio</name>
			<description>The best in Christian rock music</description>
			<logo>http://example.com/station.jpg</logo>
		</searchResult>
	</stations>
</results>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(responseXML))
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

	response, err := client.SearchStation("PANDORA", "user123", "Zach Williams")
	if err != nil {
		t.Fatalf("SearchStation failed: %v", err)
	}

	// Test basic properties
	if response.DeviceID != "1004567890AA" {
		t.Errorf("Expected deviceID '1004567890AA', got %s", response.DeviceID)
	}

	if response.Source != "PANDORA" {
		t.Errorf("Expected source PANDORA, got %s", response.Source)
	}

	// Test result categorization
	songs := response.GetSongs()
	if len(songs) != 2 {
		t.Errorf("Expected 2 songs, got %d", len(songs))
	}

	artists := response.GetArtists()
	if len(artists) != 1 {
		t.Errorf("Expected 1 artist, got %d", len(artists))
	}

	stations := response.GetStations()
	if len(stations) != 1 {
		t.Errorf("Expected 1 station, got %d", len(stations))
	}

	// Test total result count
	if response.GetResultCount() != 4 {
		t.Errorf("Expected 4 total results, got %d", response.GetResultCount())
	}

	// Test individual result properties
	song := songs[0]
	if !song.IsSong() {
		t.Error("First result should be identified as song")
	}

	if song.GetFullTitle() != "Old Church Choir - Zach Williams" {
		t.Errorf("Expected 'Old Church Choir - Zach Williams', got %s", song.GetFullTitle())
	}

	artist := artists[0]
	if !artist.IsArtist() {
		t.Error("Artist result should be identified as artist")
	}

	if artist.GetDisplayName() != "Zach Williams" {
		t.Errorf("Expected 'Zach Williams', got %s", artist.GetDisplayName())
	}

	station := stations[0]
	if !station.IsStation() {
		t.Error("Station result should be identified as station")
	}

	if station.Description == "" {
		t.Error("Station should have description")
	}

	// Test response helper methods
	allResults := response.GetAllResults()
	if len(allResults) != 4 {
		t.Errorf("Expected 4 total results, got %d", len(allResults))
	}

	if response.IsEmpty() {
		t.Error("Response should not be empty")
	}

	if !response.HasResults() {
		t.Error("Response should have results")
	}
}

func TestClient_NavigationHTTPHeaders(t *testing.T) {
	var capturedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header

		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<navigateResponse source="TUNEIN">
	<totalItems>0</totalItems>
	<items></items>
</navigateResponse>`))
	}))
	defer server.Close()

	config := &Config{
		Host:      server.URL[7:],
		Port:      80,
		Timeout:   testTimeout,
		UserAgent: "Custom-Test-Agent/1.0",
	}
	client := NewClient(config)
	client.baseURL = server.URL

	_, err := client.Navigate("TUNEIN", "", 1, 10)
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	// Verify HTTP headers
	if capturedHeaders.Get("Content-Type") != "application/xml" {
		t.Errorf("Expected Content-Type 'application/xml', got %s", capturedHeaders.Get("Content-Type"))
	}

	if capturedHeaders.Get("Accept") != "application/xml" {
		t.Errorf("Expected Accept 'application/xml', got %s", capturedHeaders.Get("Accept"))
	}

	if capturedHeaders.Get("User-Agent") != "Custom-Test-Agent/1.0" {
		t.Errorf("Expected User-Agent 'Custom-Test-Agent/1.0', got %s", capturedHeaders.Get("User-Agent"))
	}
}

func TestClient_NavigationEdgeCases(t *testing.T) {
	t.Run("NavigateContainer_NilContentItem", func(t *testing.T) {
		config := &Config{
			Host:      "localhost",
			Port:      8090,
			Timeout:   testTimeout,
			UserAgent: testUserAgent,
		}
		client := NewClient(config)

		_, err := client.NavigateContainer("STORED_MUSIC", "device/0", 1, 100, nil)
		if err == nil || !strings.Contains(err.Error(), "container item cannot be nil") {
			t.Error("Expected error for nil container item")
		}
	})

	t.Run("SearchStation_EmptySearchTerm", func(t *testing.T) {
		config := &Config{
			Host:      "localhost",
			Port:      8090,
			Timeout:   testTimeout,
			UserAgent: testUserAgent,
		}
		client := NewClient(config)

		_, err := client.SearchStation("PANDORA", "user", "")
		if err == nil || !strings.Contains(err.Error(), "search term cannot be empty") {
			t.Error("Expected error for empty search term")
		}
	})

	t.Run("AddStation_EmptyParameters", func(t *testing.T) {
		config := &Config{
			Host:      "localhost",
			Port:      8090,
			Timeout:   testTimeout,
			UserAgent: testUserAgent,
		}
		client := NewClient(config)

		// Test empty source
		err := client.AddStation("", "user", "token", "name")
		if err == nil || !strings.Contains(err.Error(), "source cannot be empty") {
			t.Error("Expected error for empty source")
		}

		// Test empty token
		err = client.AddStation("PANDORA", "user", "", "name")
		if err == nil || !strings.Contains(err.Error(), "token cannot be empty") {
			t.Error("Expected error for empty token")
		}

		// Test empty name
		err = client.AddStation("PANDORA", "user", "token", "")
		if err == nil || !strings.Contains(err.Error(), "station name cannot be empty") {
			t.Error("Expected error for empty station name")
		}
	})

	t.Run("Navigate_InvalidRange", func(t *testing.T) {
		config := &Config{
			Host:      "localhost",
			Port:      8090,
			Timeout:   testTimeout,
			UserAgent: testUserAgent,
		}
		client := NewClient(config)

		// Test invalid startItem
		_, err := client.Navigate("TUNEIN", "", 0, 10)
		if err == nil || !strings.Contains(err.Error(), "startItem must be >= 1") {
			t.Error("Expected error for invalid startItem")
		}

		// Test invalid numItems
		_, err = client.Navigate("TUNEIN", "", 1, 0)
		if err == nil || !strings.Contains(err.Error(), "numItems must be >= 1") {
			t.Error("Expected error for invalid numItems")
		}
	})
}
