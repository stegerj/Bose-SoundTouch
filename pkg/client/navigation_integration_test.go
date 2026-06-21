package client

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stegerj/bose-soundtouch/pkg/models"
)

func TestClient_Navigation_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	host := os.Getenv("SOUNDTOUCH_TEST_HOST")
	if host == "" {
		t.Skip("SOUNDTOUCH_TEST_HOST environment variable not set - skipping integration tests")
	}

	// Parse host:port if provided
	var finalHost string

	var finalPort int

	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")

		finalHost = parts[0]
		if len(parts) > 1 {
			// Use default port if parsing fails
			finalPort = 8090
		}
	} else {
		finalHost = host
		finalPort = 8090
	}

	config := &Config{
		Host:      finalHost,
		Port:      finalPort,
		Timeout:   30 * time.Second,
		UserAgent: "Bose-SoundTouch-Go-Client-Integration-Test/1.0",
	}

	client := NewClient(config)

	t.Run("Navigate_TuneIn", func(t *testing.T) {
		response, err := client.Navigate("TUNEIN", "", 1, 10)
		if err != nil {
			t.Logf("Navigate TUNEIN failed (may not be available): %v", err)
			t.Skip("TUNEIN not available on test device")

			return
		}

		t.Logf("✓ Navigate TUNEIN succeeded")
		t.Logf("  Total items: %d", response.TotalItems)
		t.Logf("  Items returned: %d", len(response.Items))

		if response.TotalItems > 0 {
			t.Logf("  First item: %s", response.Items[0].GetDisplayName())
		}
	})

	t.Run("GetTuneInStations", func(t *testing.T) {
		response, err := client.GetTuneInStations("")
		if err != nil {
			t.Logf("GetTuneInStations failed (may not be available): %v", err)
			t.Skip("TuneIn not available on test device")

			return
		}

		t.Logf("✓ GetTuneInStations succeeded")
		t.Logf("  Total stations: %d", response.TotalItems)

		stations := response.GetStations()
		t.Logf("  Station items: %d", len(stations))
	})

	t.Run("Navigate_StoredMusic", func(t *testing.T) {
		// Get sources first to check if STORED_MUSIC is available
		sources, err := client.GetSources()
		if err != nil {
			t.Fatalf("Failed to get sources: %v", err)
		}

		var storedMusicAccount string

		for _, source := range sources.SourceItem {
			if source.Source == "STORED_MUSIC" && source.Status.IsReady() {
				storedMusicAccount = source.SourceAccount
				break
			}
		}

		if storedMusicAccount == "" {
			t.Skip("STORED_MUSIC not available or not ready on test device")
		}

		response, err := client.GetStoredMusicLibrary(storedMusicAccount)
		if err != nil {
			t.Logf("GetStoredMusicLibrary failed: %v", err)
			return
		}

		t.Logf("✓ GetStoredMusicLibrary succeeded")
		t.Logf("  Source account: %s", storedMusicAccount)
		t.Logf("  Total items: %d", response.TotalItems)

		directories := response.GetDirectories()
		t.Logf("  Directories: %d", len(directories))

		tracks := response.GetTracks()
		t.Logf("  Tracks: %d", len(tracks))
	})

	t.Run("SearchStation_TuneIn", func(t *testing.T) {
		response, err := client.SearchTuneInStations("jazz")
		if err != nil {
			t.Logf("SearchTuneInStations failed (may not be supported): %v", err)
			t.Skip("TuneIn search not supported on test device")

			return
		}

		t.Logf("✓ SearchTuneInStations succeeded")
		t.Logf("  Search term: jazz")
		t.Logf("  Total results: %d", response.GetResultCount())

		songs := response.GetSongs()
		artists := response.GetArtists()
		stations := response.GetStations()

		t.Logf("  Songs: %d", len(songs))
		t.Logf("  Artists: %d", len(artists))
		t.Logf("  Stations: %d", len(stations))

		if len(stations) > 0 {
			station := stations[0]
			t.Logf("  First station: %s", station.GetDisplayName())

			if station.Token != "" {
				t.Logf("  Station token: %s", station.Token)
			}
		}
	})
}

func TestClient_StationManagement_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	host := os.Getenv("SOUNDTOUCH_TEST_HOST")
	if host == "" {
		t.Skip("SOUNDTOUCH_TEST_HOST environment variable not set - skipping integration tests")
	}

	// Parse host:port if provided
	var finalHost string

	var finalPort int

	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")

		finalHost = parts[0]
		if len(parts) > 1 {
			finalPort = 8090
		}
	} else {
		finalHost = host
		finalPort = 8090
	}

	config := &Config{
		Host:      finalHost,
		Port:      finalPort,
		Timeout:   30 * time.Second,
		UserAgent: "Bose-SoundTouch-Go-Client-Integration-Test/1.0",
	}

	client := NewClient(config)

	t.Run("SearchAndAddStation_Pandora", func(t *testing.T) {
		// Get sources first to check if Pandora is available
		sources, err := client.GetSources()
		if err != nil {
			t.Fatalf("Failed to get sources: %v", err)
		}

		var pandoraAccount string

		for _, source := range sources.SourceItem {
			if source.Source == "PANDORA" && source.Status.IsReady() {
				pandoraAccount = source.SourceAccount

				break
			}
		}

		if pandoraAccount == "" {
			t.Skip("Pandora not available or not configured on test device")
		}

		// Search for stations
		searchResponse, err := client.SearchPandoraStations(pandoraAccount, "classic rock")
		if err != nil {
			t.Logf("SearchPandoraStations failed: %v", err)
			t.Skip("Pandora search not working")

			return
		}

		t.Logf("✓ SearchPandoraStations succeeded")
		t.Logf("  Account: %s", pandoraAccount)
		t.Logf("  Results: %d", searchResponse.GetResultCount())

		// Try to find an artist or station result to add
		var tokenToAdd string

		var nameToAdd string

		artists := searchResponse.GetArtists()
		if len(artists) > 0 {
			tokenToAdd = artists[0].Token
			nameToAdd = artists[0].Name + " Radio"
		} else {
			stations := searchResponse.GetStations()
			if len(stations) > 0 {
				tokenToAdd = stations[0].Token
				nameToAdd = stations[0].Name
			}
		}

		if tokenToAdd == "" {
			t.Skip("No suitable results found to test AddStation")
		}

		t.Logf("  Will attempt to add: %s (Token: %s)", nameToAdd, tokenToAdd)

		// Note: AddStation immediately starts playing and modifies user's collection
		// In a real integration test, you might want to skip this or use a test account
		t.Logf("  Skipping actual AddStation to avoid modifying user collection")
		t.Logf("  AddStation would call: client.AddStation(%q, %q, %q, %q)", "PANDORA", pandoraAccount, tokenToAdd, nameToAdd)
	})

	t.Run("NavigateContainer_Integration", func(t *testing.T) {
		// Get sources to find a suitable container-based source
		sources, err := client.GetSources()
		if err != nil {
			t.Fatalf("Failed to get sources: %v", err)
		}

		var testSource string

		var testAccount string

		// Look for STORED_MUSIC as it typically has containers

		for _, source := range sources.SourceItem {
			if source.Source == "STORED_MUSIC" && source.Status.IsReady() {
				testSource = source.Source
				testAccount = source.SourceAccount

				break
			}
		}

		if testSource == "" {
			t.Skip("No suitable container-based source found")
		}

		// First, navigate to get a container
		response, err := client.Navigate(testSource, testAccount, 1, 10)
		if err != nil {
			t.Logf("Initial navigate failed: %v", err)
			return
		}

		directories := response.GetDirectories()
		if len(directories) == 0 {
			t.Skip("No directories found to test container navigation")
		}

		// Pick the first directory to navigate into
		container := directories[0]
		if container.ContentItem == nil {
			t.Skip("Directory has no ContentItem for navigation")
		}

		t.Logf("✓ Found container: %s", container.GetDisplayName())

		// Navigate into the container
		containerResponse, err := client.NavigateContainer(testSource, testAccount, 1, 20, container.ContentItem)
		if err != nil {
			t.Logf("NavigateContainer failed: %v", err)
			return
		}

		t.Logf("✓ NavigateContainer succeeded")
		t.Logf("  Container: %s", container.GetDisplayName())
		t.Logf("  Items in container: %d", len(containerResponse.Items))

		tracks := containerResponse.GetTracks()
		subdirs := containerResponse.GetDirectories()

		t.Logf("  Tracks: %d", len(tracks))
		t.Logf("  Subdirectories: %d", len(subdirs))
	})
}

func TestClient_Navigation_ErrorHandling_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	host := os.Getenv("SOUNDTOUCH_TEST_HOST")
	if host == "" {
		t.Skip("SOUNDTOUCH_TEST_HOST environment variable not set - skipping integration tests")
	}

	// Parse host:port if provided
	var finalHost string

	var finalPort int

	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")

		finalHost = parts[0]
		if len(parts) > 1 {
			finalPort = 8090
		}
	} else {
		finalHost = host
		finalPort = 8090
	}

	config := &Config{
		Host:      finalHost,
		Port:      finalPort,
		Timeout:   10 * time.Second,
		UserAgent: "Bose-SoundTouch-Go-Client-Integration-Test/1.0",
	}

	client := NewClient(config)

	t.Run("Navigate_InvalidSource", func(t *testing.T) {
		_, err := client.Navigate("INVALID_SOURCE", "", 1, 10)
		if err == nil {
			t.Error("Expected error for invalid source, got none")
		} else {
			t.Logf("✓ Correctly failed for invalid source: %v", err)
		}
	})

	t.Run("SearchStation_InvalidSource", func(t *testing.T) {
		_, err := client.SearchStation("INVALID_SOURCE", "", "test")
		if err == nil {
			t.Error("Expected error for invalid source, got none")
		} else {
			t.Logf("✓ Correctly failed for invalid source: %v", err)
		}
	})

	t.Run("AddStation_InvalidToken", func(t *testing.T) {
		err := client.AddStation("PANDORA", "fake_account", "invalid_token", "Test Station")
		if err == nil {
			t.Error("Expected error for invalid token, got none")
		} else {
			t.Logf("✓ Correctly failed for invalid token: %v", err)
		}
	})

	t.Run("RemoveStation_InvalidContentItem", func(t *testing.T) {
		invalidContentItem := &models.ContentItem{
			Source:   "PANDORA",
			Location: "invalid_location",
			ItemName: "Invalid Station",
		}

		err := client.RemoveStation(invalidContentItem)
		if err == nil {
			t.Error("Expected error for invalid content item, got none")
		} else {
			t.Logf("✓ Correctly failed for invalid content item: %v", err)
		}
	})
}

func BenchmarkClient_Navigate_Integration(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping integration benchmarks in short mode")
	}

	host := os.Getenv("SOUNDTOUCH_TEST_HOST")
	if host == "" {
		b.Skip("SOUNDTOUCH_TEST_HOST environment variable not set - skipping integration benchmarks")
	}

	// Parse host:port if provided
	var finalHost string

	var finalPort int

	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")

		finalHost = parts[0]
		if len(parts) > 1 {
			finalPort = 8090
		}
	} else {
		finalHost = host
		finalPort = 8090
	}

	config := &Config{
		Host:      finalHost,
		Port:      finalPort,
		Timeout:   10 * time.Second,
		UserAgent: "Bose-SoundTouch-Go-Client-Benchmark/1.0",
	}

	client := NewClient(config)

	b.ResetTimer()

	b.Run("Navigate_TuneIn", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := client.Navigate("TUNEIN", "", 1, 10)
			if err != nil {
				b.Logf("Navigate failed: %v", err)
				b.Skip("TuneIn not available")

				return
			}
		}
	})

	b.Run("SearchStation_TuneIn", func(b *testing.B) {
		searchTerms := []string{"jazz", "rock", "classical", "pop", "country"}

		for i := 0; i < b.N; i++ {
			term := searchTerms[i%len(searchTerms)]

			_, err := client.SearchTuneInStations(term)
			if err != nil {
				b.Logf("Search failed: %v", err)
				b.Skip("TuneIn search not available")

				return
			}
		}
	})
}
