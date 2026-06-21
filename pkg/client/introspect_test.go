package client

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stegerj/bose-soundtouch/pkg/models"
)

func TestClient_Introspect(t *testing.T) {
	tests := []struct {
		name          string
		source        string
		sourceAccount string
		responseXML   string
		expectedError string
		wantResponse  *models.IntrospectResponse
	}{
		{
			name:          "successful spotify introspect",
			source:        "SPOTIFY",
			sourceAccount: "SpotifyConnectUserName",
			responseXML: `<?xml version="1.0" encoding="UTF-8" ?>
<spotifyAccountIntrospectResponse state="InactiveUnselected" user="SpotifyConnectUserName" isPlaying="false" tokenLastChangedTimeSeconds="1702566495" tokenLastChangedTimeMicroseconds="427884" shuffleMode="OFF" playStatusState="2" currentUri="" receivedPlaybackRequest="false" subscriptionType="">
  <cachedPlaybackRequest />
  <nowPlaying skipPreviousSupported="false" seekSupported="false" resumeSupported="true" collectData="true" />
  <contentItemHistory maxSize="10" />
</spotifyAccountIntrospectResponse>`,
			wantResponse: &models.IntrospectResponse{
				State:                            "InactiveUnselected",
				User:                             "SpotifyConnectUserName",
				IsPlaying:                        false,
				TokenLastChangedTimeSeconds:      1702566495,
				TokenLastChangedTimeMicroseconds: 427884,
				ShuffleMode:                      "OFF",
				PlayStatusState:                  "2",
				CurrentURI:                       "",
				ReceivedPlaybackRequest:          false,
				SubscriptionType:                 "",
				CachedPlaybackRequest:            &models.CachedPlaybackRequest{},
				NowPlaying: &models.IntrospectNowPlaying{
					SkipPreviousSupported: false,
					SeekSupported:         false,
					ResumeSupported:       true,
					CollectData:           true,
				},
				ContentItemHistory: &models.ContentItemHistory{
					MaxSize: 10,
				},
			},
		},
		{
			name:          "successful pandora introspect",
			source:        "PANDORA",
			sourceAccount: "pandora_user",
			responseXML: `<?xml version="1.0" encoding="UTF-8" ?>
<pandoraAccountIntrospectResponse state="Active" user="pandora_user" isPlaying="true" shuffleMode="ON" currentUri="pandora://track/123" subscriptionType="Premium">
  <nowPlaying skipPreviousSupported="true" seekSupported="false" resumeSupported="true" collectData="false" />
  <contentItemHistory maxSize="20" />
</pandoraAccountIntrospectResponse>`,
			wantResponse: &models.IntrospectResponse{
				State:            "Active",
				User:             "pandora_user",
				IsPlaying:        true,
				ShuffleMode:      "ON",
				CurrentURI:       "pandora://track/123",
				SubscriptionType: "Premium",
				NowPlaying: &models.IntrospectNowPlaying{
					SkipPreviousSupported: true,
					SeekSupported:         false,
					ResumeSupported:       true,
					CollectData:           false,
				},
				ContentItemHistory: &models.ContentItemHistory{
					MaxSize: 20,
				},
			},
		},
		{
			name:          "empty source error",
			source:        "",
			sourceAccount: "test_user",
			expectedError: "source cannot be empty",
		},
		{
			name:          "http error",
			source:        "SPOTIFY",
			sourceAccount: "test_user",
			responseXML:   "",
			expectedError: "failed to get introspect data for SPOTIFY:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and path
				if r.Method != "POST" {
					t.Errorf("expected POST request, got %s", r.Method)
				}

				if r.URL.Path != "/introspect" {
					t.Errorf("expected /introspect path, got %s", r.URL.Path)
				}

				// Verify request body
				var requestBody models.IntrospectRequest
				if err := xml.NewDecoder(r.Body).Decode(&requestBody); err != nil {
					t.Errorf("failed to decode request body: %v", err)
				}

				if requestBody.Source != tt.source {
					t.Errorf("expected source %s, got %s", tt.source, requestBody.Source)
				}

				if requestBody.SourceAccount != tt.sourceAccount {
					t.Errorf("expected sourceAccount %s, got %s", tt.sourceAccount, requestBody.SourceAccount)
				}

				if tt.responseXML == "" {
					// Simulate server error
					w.WriteHeader(http.StatusInternalServerError)
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

			response, err := client.Introspect(tt.source, tt.sourceAccount)

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

			// Verify response fields
			if response.State != tt.wantResponse.State {
				t.Errorf("expected state %s, got %s", tt.wantResponse.State, response.State)
			}

			if response.User != tt.wantResponse.User {
				t.Errorf("expected user %s, got %s", tt.wantResponse.User, response.User)
			}

			if response.IsPlaying != tt.wantResponse.IsPlaying {
				t.Errorf("expected isPlaying %t, got %t", tt.wantResponse.IsPlaying, response.IsPlaying)
			}

			if response.ShuffleMode != tt.wantResponse.ShuffleMode {
				t.Errorf("expected shuffleMode %s, got %s", tt.wantResponse.ShuffleMode, response.ShuffleMode)
			}

			if response.CurrentURI != tt.wantResponse.CurrentURI {
				t.Errorf("expected currentUri %s, got %s", tt.wantResponse.CurrentURI, response.CurrentURI)
			}

			if response.SubscriptionType != tt.wantResponse.SubscriptionType {
				t.Errorf("expected subscriptionType %s, got %s", tt.wantResponse.SubscriptionType, response.SubscriptionType)
			}

			// Verify nested structures
			if tt.wantResponse.NowPlaying != nil {
				if response.NowPlaying == nil {
					t.Error("expected nowPlaying, got nil")
				} else {
					if response.NowPlaying.SkipPreviousSupported != tt.wantResponse.NowPlaying.SkipPreviousSupported {
						t.Errorf("expected skipPreviousSupported %t, got %t",
							tt.wantResponse.NowPlaying.SkipPreviousSupported,
							response.NowPlaying.SkipPreviousSupported)
					}

					if response.NowPlaying.SeekSupported != tt.wantResponse.NowPlaying.SeekSupported {
						t.Errorf("expected seekSupported %t, got %t",
							tt.wantResponse.NowPlaying.SeekSupported,
							response.NowPlaying.SeekSupported)
					}

					if response.NowPlaying.ResumeSupported != tt.wantResponse.NowPlaying.ResumeSupported {
						t.Errorf("expected resumeSupported %t, got %t",
							tt.wantResponse.NowPlaying.ResumeSupported,
							response.NowPlaying.ResumeSupported)
					}

					if response.NowPlaying.CollectData != tt.wantResponse.NowPlaying.CollectData {
						t.Errorf("expected collectData %t, got %t",
							tt.wantResponse.NowPlaying.CollectData,
							response.NowPlaying.CollectData)
					}
				}
			}

			if tt.wantResponse.ContentItemHistory != nil {
				if response.ContentItemHistory == nil {
					t.Error("expected contentItemHistory, got nil")
				} else {
					if response.ContentItemHistory.MaxSize != tt.wantResponse.ContentItemHistory.MaxSize {
						t.Errorf("expected maxSize %d, got %d",
							tt.wantResponse.ContentItemHistory.MaxSize,
							response.ContentItemHistory.MaxSize)
					}
				}
			}
		})
	}
}

func TestIntrospectResponse_Methods(t *testing.T) {
	response := &models.IntrospectResponse{
		State:            "Active",
		User:             "test_user",
		IsPlaying:        true,
		ShuffleMode:      "ON",
		CurrentURI:       "spotify://track/123",
		SubscriptionType: "Premium",
		NowPlaying: &models.IntrospectNowPlaying{
			SkipPreviousSupported: true,
			SeekSupported:         true,
			ResumeSupported:       true,
			CollectData:           false,
		},
		ContentItemHistory: &models.ContentItemHistory{
			MaxSize: 15,
		},
	}

	// Test state methods
	if !response.IsActive() {
		t.Error("expected IsActive() to return true")
	}

	if response.IsInactive() {
		t.Error("expected IsInactive() to return false")
	}

	// Test user methods
	if !response.HasUser() {
		t.Error("expected HasUser() to return true")
	}

	// Test shuffle methods
	if !response.IsShuffleEnabled() {
		t.Error("expected IsShuffleEnabled() to return true")
	}

	// Test content methods
	if !response.HasCurrentContent() {
		t.Error("expected HasCurrentContent() to return true")
	}

	// Test capability methods
	if !response.SupportsSkipPrevious() {
		t.Error("expected SupportsSkipPrevious() to return true")
	}

	if !response.SupportsSeek() {
		t.Error("expected SupportsSeek() to return true")
	}

	if !response.SupportsResume() {
		t.Error("expected SupportsResume() to return true")
	}

	if response.CollectsData() {
		t.Error("expected CollectsData() to return false")
	}

	// Test history methods
	if response.GetMaxHistorySize() != 15 {
		t.Errorf("expected GetMaxHistorySize() to return 15, got %d", response.GetMaxHistorySize())
	}

	// Test subscription methods
	if !response.HasSubscription() {
		t.Error("expected HasSubscription() to return true")
	}
}

func TestIntrospectResponse_InactiveState(t *testing.T) {
	response := &models.IntrospectResponse{
		State:            "InactiveUnselected",
		User:             "",
		IsPlaying:        false,
		ShuffleMode:      "OFF",
		CurrentURI:       "",
		SubscriptionType: "",
	}

	// Test inactive state
	if response.IsActive() {
		t.Error("expected IsActive() to return false")
	}

	if !response.IsInactive() {
		t.Error("expected IsInactive() to return true")
	}

	// Test empty values
	if response.HasUser() {
		t.Error("expected HasUser() to return false")
	}

	if response.IsShuffleEnabled() {
		t.Error("expected IsShuffleEnabled() to return false")
	}

	if response.HasCurrentContent() {
		t.Error("expected HasCurrentContent() to return false")
	}

	if response.HasSubscription() {
		t.Error("expected HasSubscription() to return false")
	}
}

func TestNewIntrospectRequest(t *testing.T) {
	tests := []struct {
		name          string
		source        string
		sourceAccount string
	}{
		{
			name:          "with source account",
			source:        "SPOTIFY",
			sourceAccount: "test_user",
		},
		{
			name:          "without source account",
			source:        "BLUETOOTH",
			sourceAccount: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := models.NewIntrospectRequest(tt.source, tt.sourceAccount)

			if request == nil {
				t.Error("expected request, got nil")
				return
			}

			if request.Source != tt.source {
				t.Errorf("expected source %s, got %s", tt.source, request.Source)
			}

			if request.SourceAccount != tt.sourceAccount {
				t.Errorf("expected sourceAccount %s, got %s", tt.sourceAccount, request.SourceAccount)
			}
		})
	}
}
