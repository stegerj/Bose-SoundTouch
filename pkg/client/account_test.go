package client

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stegerj/bose-soundtouch/pkg/models"
)

func TestClient_SetMusicServiceAccount(t *testing.T) {
	tests := []struct {
		name         string
		credentials  *models.MusicServiceCredentials
		serverStatus int
		serverBody   string
		wantError    bool
		errorMessage string
	}{
		{
			name:         "Valid Spotify credentials",
			credentials:  models.NewSpotifyCredentials("user@spotify.com", "mypassword"),
			serverStatus: http.StatusOK,
			serverBody:   `<?xml version="1.0" encoding="UTF-8" ?><status>/setMusicServiceAccount</status>`,
			wantError:    false,
		},
		{
			name:         "Valid Pandora credentials",
			credentials:  models.NewPandoraCredentials("pandora_user", "pandora_pass"),
			serverStatus: http.StatusOK,
			serverBody:   `<?xml version="1.0" encoding="UTF-8" ?><status>/setMusicServiceAccount</status>`,
			wantError:    false,
		},
		{
			name:         "Valid STORED_MUSIC credentials",
			credentials:  models.NewStoredMusicCredentials("d09708a1-5953-44bc-a413-123456789012/0", "My NAS Library"),
			serverStatus: http.StatusOK,
			serverBody:   `<?xml version="1.0" encoding="UTF-8" ?><status>/setMusicServiceAccount</status>`,
			wantError:    false,
		},
		{
			name:         "Nil credentials",
			credentials:  nil,
			wantError:    true,
			errorMessage: "credentials cannot be nil",
		},
		{
			name: "Invalid credentials - empty source",
			credentials: &models.MusicServiceCredentials{
				Source:      "",
				DisplayName: "Test Service",
				User:        "testuser",
				Pass:        "testpass",
			},
			wantError:    true,
			errorMessage: "invalid credentials: source cannot be empty",
		},
		{
			name: "Invalid credentials - empty user",
			credentials: &models.MusicServiceCredentials{
				Source:      "SPOTIFY",
				DisplayName: "Spotify",
				User:        "",
				Pass:        "testpass",
			},
			wantError:    true,
			errorMessage: "invalid credentials: user cannot be empty",
		},
		{
			name: "Invalid credentials - empty password for non-STORED_MUSIC",
			credentials: &models.MusicServiceCredentials{
				Source:      "SPOTIFY",
				DisplayName: "Spotify",
				User:        "testuser",
				Pass:        "",
			},
			wantError:    true,
			errorMessage: "invalid credentials: password cannot be empty for SPOTIFY",
		},
		{
			name:         "Server error",
			credentials:  models.NewSpotifyCredentials("user@spotify.com", "mypassword"),
			serverStatus: http.StatusInternalServerError,
			serverBody:   "Internal Server Error",
			wantError:    true,
			errorMessage: "failed to set music service account for SPOTIFY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedRequest *models.MusicServiceCredentials

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/setMusicServiceAccount" {
					t.Errorf("Expected path /setMusicServiceAccount, got %s", r.URL.Path)
				}

				if r.Method != "POST" {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				// Parse request body to verify credentials
				if tt.credentials != nil {
					var req models.MusicServiceCredentials
					if err := xml.NewDecoder(r.Body).Decode(&req); err == nil {
						receivedRequest = &req
					}
				}

				w.WriteHeader(tt.serverStatus)

				if tt.serverBody != "" {
					_, _ = w.Write([]byte(tt.serverBody))
				}
			}))
			defer server.Close()

			config := &Config{
				Host:    server.URL[7:],
				Port:    80,
				Timeout: testTimeout,
			}
			client := NewClient(config)
			client.baseURL = server.URL

			err := client.SetMusicServiceAccount(tt.credentials)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMessage != "" && !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("Expected error message to contain %q, got %q", tt.errorMessage, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Verify request was sent correctly
				if receivedRequest != nil {
					if receivedRequest.Source != tt.credentials.Source {
						t.Errorf("Expected source %s, got %s", tt.credentials.Source, receivedRequest.Source)
					}

					if receivedRequest.User != tt.credentials.User {
						t.Errorf("Expected user %s, got %s", tt.credentials.User, receivedRequest.User)
					}

					if receivedRequest.Pass != tt.credentials.Pass {
						t.Errorf("Expected pass %s, got %s", tt.credentials.Pass, receivedRequest.Pass)
					}
				}
			}
		})
	}
}

func TestClient_RemoveMusicServiceAccount(t *testing.T) {
	tests := []struct {
		name         string
		credentials  *models.MusicServiceCredentials
		serverStatus int
		serverBody   string
		wantError    bool
		errorMessage string
	}{
		{
			name:         "Valid Spotify removal",
			credentials:  models.NewSpotifyCredentials("user@spotify.com", "mypassword"),
			serverStatus: http.StatusOK,
			serverBody:   `<?xml version="1.0" encoding="UTF-8" ?><status>/removeMusicServiceAccount</status>`,
			wantError:    false,
		},
		{
			name:         "Valid Pandora removal",
			credentials:  models.NewPandoraCredentials("pandora_user", "pandora_pass"),
			serverStatus: http.StatusOK,
			serverBody:   `<?xml version="1.0" encoding="UTF-8" ?><status>/removeMusicServiceAccount</status>`,
			wantError:    false,
		},
		{
			name:         "Nil credentials",
			credentials:  nil,
			wantError:    true,
			errorMessage: "credentials cannot be nil",
		},
		{
			name: "Empty source",
			credentials: &models.MusicServiceCredentials{
				Source: "",
				User:   "testuser",
			},
			wantError:    true,
			errorMessage: "source cannot be empty",
		},
		{
			name: "Empty user",
			credentials: &models.MusicServiceCredentials{
				Source: "SPOTIFY",
				User:   "",
			},
			wantError:    true,
			errorMessage: "user cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedRequest *models.MusicServiceCredentials

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/removeMusicServiceAccount" {
					t.Errorf("Expected path /removeMusicServiceAccount, got %s", r.URL.Path)
				}

				// Parse request body to verify credentials have empty password
				if tt.credentials != nil {
					var req models.MusicServiceCredentials
					if err := xml.NewDecoder(r.Body).Decode(&req); err == nil {
						receivedRequest = &req
					}
				}

				w.WriteHeader(tt.serverStatus)

				if tt.serverBody != "" {
					_, _ = w.Write([]byte(tt.serverBody))
				}
			}))
			defer server.Close()

			config := &Config{
				Host:    server.URL[7:],
				Port:    80,
				Timeout: testTimeout,
			}
			client := NewClient(config)
			client.baseURL = server.URL

			err := client.RemoveMusicServiceAccount(tt.credentials)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMessage != "" && !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("Expected error message to contain %q, got %q", tt.errorMessage, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Verify password was cleared for removal
				if receivedRequest != nil && receivedRequest.Pass != "" {
					t.Errorf("Expected empty password for removal, got %s", receivedRequest.Pass)
				}
			}
		})
	}
}

func TestClient_AddSpotifyAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/setMusicServiceAccount" {
			t.Errorf("Expected path /setMusicServiceAccount, got %s", r.URL.Path)
		}

		var req models.MusicServiceCredentials
		if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if req.Source != "SPOTIFY" {
			t.Errorf("Expected source SPOTIFY, got %s", req.Source)
		}

		if req.User != "test@spotify.com" {
			t.Errorf("Expected user test@spotify.com, got %s", req.User)
		}

		if req.Pass != "mypassword" {
			t.Errorf("Expected password mypassword, got %s", req.Pass)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?><status>/setMusicServiceAccount</status>`))
	}))
	defer server.Close()

	config := &Config{
		Host:    server.URL[7:],
		Port:    80,
		Timeout: testTimeout,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	err := client.AddSpotifyAccount("test@spotify.com", "mypassword")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_RemoveSpotifyAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/removeMusicServiceAccount" {
			t.Errorf("Expected path /removeMusicServiceAccount, got %s", r.URL.Path)
		}

		var req models.MusicServiceCredentials
		if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if req.Source != "SPOTIFY" {
			t.Errorf("Expected source SPOTIFY, got %s", req.Source)
		}

		if req.User != "test@spotify.com" {
			t.Errorf("Expected user test@spotify.com, got %s", req.User)
		}

		if req.Pass != "" {
			t.Errorf("Expected empty password for removal, got %s", req.Pass)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?><status>/removeMusicServiceAccount</status>`))
	}))
	defer server.Close()

	config := &Config{
		Host:    server.URL[7:],
		Port:    80,
		Timeout: testTimeout,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	err := client.RemoveSpotifyAccount("test@spotify.com")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_AddStoredMusicAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req models.MusicServiceCredentials
		if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if req.Source != "STORED_MUSIC" {
			t.Errorf("Expected source STORED_MUSIC, got %s", req.Source)
		}

		if req.User != "d09708a1-5953-44bc-a413-123456789012/0" {
			t.Errorf("Expected NAS user ID, got %s", req.User)
		}

		if req.DisplayName != "My NAS Library" {
			t.Errorf("Expected display name 'My NAS Library', got %s", req.DisplayName)
		}

		// STORED_MUSIC should have empty password
		if req.Pass != "" {
			t.Errorf("Expected empty password for STORED_MUSIC, got %s", req.Pass)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?><status>/setMusicServiceAccount</status>`))
	}))
	defer server.Close()

	config := &Config{
		Host:    server.URL[7:],
		Port:    80,
		Timeout: testTimeout,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	err := client.AddStoredMusicAccount("d09708a1-5953-44bc-a413-123456789012/0", "My NAS Library")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_AccountManagementErrors(t *testing.T) {
	// Test network error
	client := NewClient(&Config{
		Host:    "non-existent-host.invalid",
		Port:    8090,
		Timeout: 1 * time.Second,
	})

	credentials := models.NewSpotifyCredentials("user@spotify.com", "password")

	err := client.SetMusicServiceAccount(credentials)
	if err == nil {
		t.Error("Expected error for network error")
	}

	err = client.RemoveMusicServiceAccount(credentials)
	if err == nil {
		t.Error("Expected error for network error")
	}
}

// Test convenience methods for all supported services
func TestClient_AddAmazonMusicAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/setMusicServiceAccount" {
			t.Errorf("Expected path /setMusicServiceAccount, got %s", r.URL.Path)
		}

		var req models.MusicServiceCredentials
		if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if req.Source != "AMAZON" {
			t.Errorf("Expected source AMAZON, got %s", req.Source)
		}

		if req.User != "test@amazon.com" {
			t.Errorf("Expected user test@amazon.com, got %s", req.User)
		}

		if req.Pass != "mypassword" {
			t.Errorf("Expected password mypassword, got %s", req.Pass)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?><status>/setMusicServiceAccount</status>`))
	}))
	defer server.Close()

	config := &Config{
		Host:    server.URL[7:],
		Port:    80,
		Timeout: testTimeout,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	err := client.AddAmazonMusicAccount("test@amazon.com", "mypassword")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_RemoveAmazonMusicAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/removeMusicServiceAccount" {
			t.Errorf("Expected path /removeMusicServiceAccount, got %s", r.URL.Path)
		}

		var req models.MusicServiceCredentials
		if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if req.Source != "AMAZON" {
			t.Errorf("Expected source AMAZON, got %s", req.Source)
		}

		if req.User != "test@amazon.com" {
			t.Errorf("Expected user test@amazon.com, got %s", req.User)
		}

		if req.Pass != "" {
			t.Errorf("Expected empty password for removal, got %s", req.Pass)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?><status>/removeMusicServiceAccount</status>`))
	}))
	defer server.Close()

	config := &Config{
		Host:    server.URL[7:],
		Port:    80,
		Timeout: testTimeout,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	err := client.RemoveAmazonMusicAccount("test@amazon.com")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_AddDeezerAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/setMusicServiceAccount" {
			t.Errorf("Expected path /setMusicServiceAccount, got %s", r.URL.Path)
		}

		var req models.MusicServiceCredentials
		if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if req.Source != "DEEZER" {
			t.Errorf("Expected source DEEZER, got %s", req.Source)
		}

		if req.User != "deezer_user" {
			t.Errorf("Expected user deezer_user, got %s", req.User)
		}

		if req.Pass != "deezer_pass" {
			t.Errorf("Expected password deezer_pass, got %s", req.Pass)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?><status>/setMusicServiceAccount</status>`))
	}))
	defer server.Close()

	config := &Config{
		Host:    server.URL[7:],
		Port:    80,
		Timeout: testTimeout,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	err := client.AddDeezerAccount("deezer_user", "deezer_pass")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_RemoveDeezerAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/removeMusicServiceAccount" {
			t.Errorf("Expected path /removeMusicServiceAccount, got %s", r.URL.Path)
		}

		var req models.MusicServiceCredentials
		if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if req.Source != "DEEZER" {
			t.Errorf("Expected source DEEZER, got %s", req.Source)
		}

		if req.User != "deezer_user" {
			t.Errorf("Expected user deezer_user, got %s", req.User)
		}

		if req.Pass != "" {
			t.Errorf("Expected empty password for removal, got %s", req.Pass)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?><status>/removeMusicServiceAccount</status>`))
	}))
	defer server.Close()

	config := &Config{
		Host:    server.URL[7:],
		Port:    80,
		Timeout: testTimeout,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	err := client.RemoveDeezerAccount("deezer_user")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_AddIHeartRadioAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/setMusicServiceAccount" {
			t.Errorf("Expected path /setMusicServiceAccount, got %s", r.URL.Path)
		}

		var req models.MusicServiceCredentials
		if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if req.Source != "IHEART" {
			t.Errorf("Expected source IHEART, got %s", req.Source)
		}

		if req.User != "iheart_user" {
			t.Errorf("Expected user iheart_user, got %s", req.User)
		}

		if req.Pass != "iheart_pass" {
			t.Errorf("Expected password iheart_pass, got %s", req.Pass)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?><status>/setMusicServiceAccount</status>`))
	}))
	defer server.Close()

	config := &Config{
		Host:    server.URL[7:],
		Port:    80,
		Timeout: testTimeout,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	err := client.AddIHeartRadioAccount("iheart_user", "iheart_pass")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_RemoveIHeartRadioAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/removeMusicServiceAccount" {
			t.Errorf("Expected path /removeMusicServiceAccount, got %s", r.URL.Path)
		}

		var req models.MusicServiceCredentials
		if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if req.Source != "IHEART" {
			t.Errorf("Expected source IHEART, got %s", req.Source)
		}

		if req.User != "iheart_user" {
			t.Errorf("Expected user iheart_user, got %s", req.User)
		}

		if req.Pass != "" {
			t.Errorf("Expected empty password for removal, got %s", req.Pass)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" ?><status>/removeMusicServiceAccount</status>`))
	}))
	defer server.Close()

	config := &Config{
		Host:    server.URL[7:],
		Port:    80,
		Timeout: testTimeout,
	}
	client := NewClient(config)
	client.baseURL = server.URL

	err := client.RemoveIHeartRadioAccount("iheart_user")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_ConvenienceMethodsExist(_ *testing.T) {
	client := NewClient(&Config{
		Host:    "localhost",
		Port:    8090,
		Timeout: testTimeout,
	})

	// Test that convenience methods exist (compilation test)
	var err error

	// Spotify
	err = client.AddSpotifyAccount("user", "pass")
	_ = err // Expect network error, but method should exist

	err = client.RemoveSpotifyAccount("user")
	_ = err

	// Pandora
	err = client.AddPandoraAccount("user", "pass")
	_ = err

	err = client.RemovePandoraAccount("user")
	_ = err

	// Amazon Music
	err = client.AddAmazonMusicAccount("user", "pass")
	_ = err

	err = client.RemoveAmazonMusicAccount("user")
	_ = err

	// Deezer
	err = client.AddDeezerAccount("user", "pass")
	_ = err

	err = client.RemoveDeezerAccount("user")
	_ = err

	// iHeartRadio
	err = client.AddIHeartRadioAccount("user", "pass")
	_ = err

	err = client.RemoveIHeartRadioAccount("user")
	_ = err

	// STORED_MUSIC
	err = client.AddStoredMusicAccount("guid/0", "Display Name")
	_ = err

	err = client.RemoveStoredMusicAccount("guid/0", "Display Name")
	_ = err
}
