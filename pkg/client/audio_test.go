package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stegerj/bose-soundtouch/pkg/models"
)

func TestClient_GetAudioDSPControls(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		expectError    bool
		expectedDSP    *models.AudioDSPControls
	}{
		{
			name:           "successful DSP controls retrieval",
			responseStatus: http.StatusOK,
			responseBody:   `<audiodspcontrols audiomode="MUSIC" videosyncaudiodelay="50" supportedaudiomodes="NORMAL|DIALOG|SURROUND|MUSIC"/>`,
			expectError:    false,
			expectedDSP: &models.AudioDSPControls{
				AudioMode:           "MUSIC",
				VideoSyncAudioDelay: 50,
				SupportedAudioModes: "NORMAL|DIALOG|SURROUND|MUSIC",
			},
		},
		{
			name:           "server error response",
			responseStatus: http.StatusInternalServerError,
			responseBody:   `<error>Internal Server Error</error>`,
			expectError:    true,
		},
		{
			name:           "not found response",
			responseStatus: http.StatusNotFound,
			responseBody:   `<error>Feature not supported</error>`,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/capabilities" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`<capabilities><capability name="audiodspcontrols"/></capabilities>`))

					return
				}

				if r.Method != "GET" {
					t.Errorf("Expected GET request, got %s", r.Method)
				}

				if r.URL.Path != "/audiodspcontrols" {
					t.Errorf("Expected path /audiodspcontrols, got %s", r.URL.Path)
				}

				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := createTestClient(server.URL)
			dspControls, err := client.GetAudioDSPControls()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}

				return
			}

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
				return
			}

			if dspControls.AudioMode != tt.expectedDSP.AudioMode {
				t.Errorf("Expected AudioMode %s, got %s", tt.expectedDSP.AudioMode, dspControls.AudioMode)
			}

			if dspControls.VideoSyncAudioDelay != tt.expectedDSP.VideoSyncAudioDelay {
				t.Errorf("Expected VideoSyncAudioDelay %d, got %d", tt.expectedDSP.VideoSyncAudioDelay, dspControls.VideoSyncAudioDelay)
			}

			if dspControls.SupportedAudioModes != tt.expectedDSP.SupportedAudioModes {
				t.Errorf("Expected SupportedAudioModes %s, got %s", tt.expectedDSP.SupportedAudioModes, dspControls.SupportedAudioModes)
			}
		})
	}
}

func TestClient_SetAudioDSPControls(t *testing.T) {
	tests := []struct {
		name           string
		audioMode      string
		videoSyncDelay int
		responseStatus int
		responseBody   string
		expectError    bool
	}{
		{
			name:           "successful DSP controls update",
			audioMode:      "MUSIC",
			videoSyncDelay: 50,
			responseStatus: http.StatusOK,
			responseBody:   `<status>OK</status>`,
			expectError:    false,
		},
		{
			name:           "audio mode only",
			audioMode:      "DIALOG",
			videoSyncDelay: 0,
			responseStatus: http.StatusOK,
			responseBody:   `<status>OK</status>`,
			expectError:    false,
		},
		{
			name:           "server error response",
			audioMode:      "MUSIC",
			videoSyncDelay: 25,
			responseStatus: http.StatusBadRequest,
			responseBody:   `<error>Bad Request</error>`,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++

				// Handle capabilities check
				if r.URL.Path == "/capabilities" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`<capabilities><capability name="audiodspcontrols"/></capabilities>`))

					return
				}

				// First call might be GET for validation
				if r.Method == "GET" && r.URL.Path == "/audiodspcontrols" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`<audiodspcontrols audiomode="NORMAL" videosyncaudiodelay="0" supportedaudiomodes="NORMAL|DIALOG|SURROUND|MUSIC"/>`))

					return
				}

				// POST call for setting
				if r.Method != "POST" {
					t.Errorf("Expected POST request, got %s", r.Method)
				}

				if r.URL.Path != "/audiodspcontrols" {
					t.Errorf("Expected path /audiodspcontrols, got %s", r.URL.Path)
				}

				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := createTestClient(server.URL)
			err := client.SetAudioDSPControls(tt.audioMode, tt.videoSyncDelay)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestClient_SetAudioMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle capabilities check
		if r.URL.Path == "/capabilities" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<capabilities><capability name="audiodspcontrols"/></capabilities>`))

			return
		}

		// First call might be GET for validation
		if r.Method == "GET" && r.URL.Path == "/audiodspcontrols" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<audiodspcontrols audiomode="NORMAL" videosyncaudiodelay="0" supportedaudiomodes="NORMAL|DIALOG|SURROUND|MUSIC"/>`))

			return
		}

		// POST call for setting
		if r.Method == "POST" && r.URL.Path == "/audiodspcontrols" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<status>OK</status>`))

			return
		}

		t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := createTestClient(server.URL)

	err := client.SetAudioMode("MUSIC")
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
}

func TestClient_SetVideoSyncAudioDelay(t *testing.T) {
	tests := []struct {
		name        string
		delay       int
		expectError bool
	}{
		{
			name:        "valid delay",
			delay:       50,
			expectError: false,
		},
		{
			name:        "zero delay",
			delay:       0,
			expectError: false,
		},
		{
			name:        "negative delay should fail",
			delay:       -10,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectError {
				// For error cases, we don't need a server
				config := DefaultConfig()
				config.Host = "localhost"
				client := NewClient(config)

				err := client.SetVideoSyncAudioDelay(tt.delay)
				if err == nil {
					t.Errorf("Expected error but got none")
				}

				return
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`<status>OK</status>`))
			}))
			defer server.Close()

			client := createTestClient(server.URL)

			err := client.SetVideoSyncAudioDelay(tt.delay)
			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestClient_GetAudioProductToneControls(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		expectError    bool
		expectedTone   *models.AudioProductToneControls
	}{
		{
			name:           "successful tone controls retrieval",
			responseStatus: http.StatusOK,
			responseBody: `<audioproducttonecontrols>
				<bass value="3" minValue="-10" maxValue="10" step="1"/>
				<treble value="-2" minValue="-5" maxValue="5" step="1"/>
			</audioproducttonecontrols>`,
			expectError: false,
			expectedTone: &models.AudioProductToneControls{
				Bass: models.BassControlSetting{
					Value:    3,
					MinValue: -10,
					MaxValue: 10,
					Step:     1,
				},
				Treble: models.TrebleControlSetting{
					Value:    -2,
					MinValue: -5,
					MaxValue: 5,
					Step:     1,
				},
			},
		},
		{
			name:           "server error response",
			responseStatus: http.StatusInternalServerError,
			responseBody:   `<error>Internal Server Error</error>`,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/capabilities" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`<capabilities><capability name="audioproducttonecontrols"/></capabilities>`))

					return
				}

				if r.Method != "GET" {
					t.Errorf("Expected GET request, got %s", r.Method)
				}

				if r.URL.Path != "/audioproducttonecontrols" {
					t.Errorf("Expected path /audioproducttonecontrols, got %s", r.URL.Path)
				}

				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := createTestClient(server.URL)
			toneControls, err := client.GetAudioProductToneControls()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}

				return
			}

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
				return
			}

			if toneControls.Bass.Value != tt.expectedTone.Bass.Value {
				t.Errorf("Expected Bass.Value %d, got %d", tt.expectedTone.Bass.Value, toneControls.Bass.Value)
			}

			if toneControls.Treble.Value != tt.expectedTone.Treble.Value {
				t.Errorf("Expected Treble.Value %d, got %d", tt.expectedTone.Treble.Value, toneControls.Treble.Value)
			}
		})
	}
}

func TestClient_SetAudioProductToneControls(t *testing.T) {
	tests := []struct {
		name           string
		bass           *int
		treble         *int
		responseStatus int
		responseBody   string
		expectError    bool
	}{
		{
			name:           "set bass and treble",
			bass:           intPtr(5),
			treble:         intPtr(-2),
			responseStatus: http.StatusOK,
			responseBody:   `<status>OK</status>`,
			expectError:    false,
		},
		{
			name:           "set bass only",
			bass:           intPtr(3),
			treble:         nil,
			responseStatus: http.StatusOK,
			responseBody:   `<status>OK</status>`,
			expectError:    false,
		},
		{
			name:           "set treble only",
			bass:           nil,
			treble:         intPtr(-1),
			responseStatus: http.StatusOK,
			responseBody:   `<status>OK</status>`,
			expectError:    false,
		},
		{
			name:           "server error response",
			bass:           intPtr(5),
			treble:         intPtr(-2),
			responseStatus: http.StatusBadRequest,
			responseBody:   `<error>Bad Request</error>`,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Handle capabilities check
				if r.URL.Path == "/capabilities" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`<capabilities><capability name="audioproducttonecontrols"/></capabilities>`))

					return
				}

				// First call might be GET for validation
				if r.Method == "GET" && r.URL.Path == "/audioproducttonecontrols" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`<audioproducttonecontrols>
						<bass value="0" minValue="-10" maxValue="10" step="1"/>
						<treble value="0" minValue="-5" maxValue="5" step="1"/>
					</audioproducttonecontrols>`))

					return
				}

				// POST call for setting
				if r.Method != "POST" {
					t.Errorf("Expected POST request, got %s", r.Method)
				}

				if r.URL.Path != "/audioproducttonecontrols" {
					t.Errorf("Expected path /audioproducttonecontrols, got %s", r.URL.Path)
				}

				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := createTestClient(server.URL)
			err := client.SetAudioProductToneControls(tt.bass, tt.treble)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestClient_SetAdvancedBass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle capabilities check
		if r.URL.Path == "/capabilities" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<capabilities><capability name="audioproducttonecontrols"/></capabilities>`))

			return
		}

		// First call might be GET for validation
		if r.Method == "GET" && r.URL.Path == "/audioproducttonecontrols" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<audioproducttonecontrols>
				<bass value="0" minValue="-10" maxValue="10" step="1"/>
				<treble value="0" minValue="-5" maxValue="5" step="1"/>
			</audioproducttonecontrols>`))

			return
		}

		// POST call for setting
		if r.Method == "POST" && r.URL.Path == "/audioproducttonecontrols" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<status>OK</status>`))

			return
		}

		t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := createTestClient(server.URL)

	err := client.SetAdvancedBass(5)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
}

func TestClient_SetAdvancedTreble(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle capabilities check
		if r.URL.Path == "/capabilities" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<capabilities><capability name="audioproducttonecontrols"/></capabilities>`))

			return
		}

		// First call might be GET for validation
		if r.Method == "GET" && r.URL.Path == "/audioproducttonecontrols" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<audioproducttonecontrols>
				<bass value="0" minValue="-10" maxValue="10" step="1"/>
				<treble value="0" minValue="-5" maxValue="5" step="1"/>
			</audioproducttonecontrols>`))

			return
		}

		// POST call for setting
		if r.Method == "POST" && r.URL.Path == "/audioproducttonecontrols" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<status>OK</status>`))

			return
		}

		t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := createTestClient(server.URL)

	err := client.SetAdvancedTreble(-2)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
}

func TestClient_GetAudioProductLevelControls(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		expectError    bool
		expectedLevel  *models.AudioProductLevelControls
	}{
		{
			name:           "successful level controls retrieval",
			responseStatus: http.StatusOK,
			responseBody: `<audioproductlevelcontrols>
				<frontCenterSpeakerLevel value="2" minValue="-10" maxValue="10" step="1"/>
				<rearSurroundSpeakersLevel value="-1" minValue="-8" maxValue="8" step="1"/>
			</audioproductlevelcontrols>`,
			expectError: false,
			expectedLevel: &models.AudioProductLevelControls{
				FrontCenterSpeakerLevel: models.FrontCenterLevelSetting{
					Value:    2,
					MinValue: -10,
					MaxValue: 10,
					Step:     1,
				},
				RearSurroundSpeakersLevel: models.RearSurroundLevelSetting{
					Value:    -1,
					MinValue: -8,
					MaxValue: 8,
					Step:     1,
				},
			},
		},
		{
			name:           "server error response",
			responseStatus: http.StatusInternalServerError,
			responseBody:   `<error>Internal Server Error</error>`,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/capabilities" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`<capabilities><capability name="audioproductlevelcontrols"/></capabilities>`))

					return
				}

				if r.Method != "GET" {
					t.Errorf("Expected GET request, got %s", r.Method)
				}

				if r.URL.Path != "/audioproductlevelcontrols" {
					t.Errorf("Expected path /audioproductlevelcontrols, got %s", r.URL.Path)
				}

				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := createTestClient(server.URL)
			levelControls, err := client.GetAudioProductLevelControls()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}

				return
			}

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
				return
			}

			if levelControls.FrontCenterSpeakerLevel.Value != tt.expectedLevel.FrontCenterSpeakerLevel.Value {
				t.Errorf("Expected FrontCenterSpeakerLevel.Value %d, got %d",
					tt.expectedLevel.FrontCenterSpeakerLevel.Value, levelControls.FrontCenterSpeakerLevel.Value)
			}

			if levelControls.RearSurroundSpeakersLevel.Value != tt.expectedLevel.RearSurroundSpeakersLevel.Value {
				t.Errorf("Expected RearSurroundSpeakersLevel.Value %d, got %d",
					tt.expectedLevel.RearSurroundSpeakersLevel.Value, levelControls.RearSurroundSpeakersLevel.Value)
			}
		})
	}
}

func TestClient_SetAudioProductLevelControls(t *testing.T) {
	tests := []struct {
		name           string
		frontCenter    *int
		rearSurround   *int
		responseStatus int
		responseBody   string
		expectError    bool
	}{
		{
			name:           "set both levels",
			frontCenter:    intPtr(3),
			rearSurround:   intPtr(-2),
			responseStatus: http.StatusOK,
			responseBody:   `<status>OK</status>`,
			expectError:    false,
		},
		{
			name:           "set front center only",
			frontCenter:    intPtr(5),
			rearSurround:   nil,
			responseStatus: http.StatusOK,
			responseBody:   `<status>OK</status>`,
			expectError:    false,
		},
		{
			name:           "set rear surround only",
			frontCenter:    nil,
			rearSurround:   intPtr(-3),
			responseStatus: http.StatusOK,
			responseBody:   `<status>OK</status>`,
			expectError:    false,
		},
		{
			name:           "server error response",
			frontCenter:    intPtr(3),
			rearSurround:   intPtr(-2),
			responseStatus: http.StatusBadRequest,
			responseBody:   `<error>Bad Request</error>`,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Handle capabilities check
				if r.URL.Path == "/capabilities" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`<capabilities><capability name="audioproductlevelcontrols"/></capabilities>`))

					return
				}

				// First call might be GET for validation
				if r.Method == "GET" && r.URL.Path == "/audioproductlevelcontrols" {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`<audioproductlevelcontrols>
						<frontCenterSpeakerLevel value="0" minValue="-10" maxValue="10" step="1"/>
						<rearSurroundSpeakersLevel value="0" minValue="-8" maxValue="8" step="1"/>
					</audioproductlevelcontrols>`))

					return
				}

				// POST call for setting
				if r.Method != "POST" {
					t.Errorf("Expected POST request, got %s", r.Method)
				}

				if r.URL.Path != "/audioproductlevelcontrols" {
					t.Errorf("Expected path /audioproductlevelcontrols, got %s", r.URL.Path)
				}

				w.WriteHeader(tt.responseStatus)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := createTestClient(server.URL)
			err := client.SetAudioProductLevelControls(tt.frontCenter, tt.rearSurround)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestClient_SetFrontCenterSpeakerLevel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle capabilities check
		if r.URL.Path == "/capabilities" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<capabilities><capability name="audioproductlevelcontrols"/></capabilities>`))

			return
		}

		// First call might be GET for validation
		if r.Method == "GET" && r.URL.Path == "/audioproductlevelcontrols" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<audioproductlevelcontrols>
				<frontCenterSpeakerLevel value="0" minValue="-10" maxValue="10" step="1"/>
				<rearSurroundSpeakersLevel value="0" minValue="-8" maxValue="8" step="1"/>
			</audioproductlevelcontrols>`))

			return
		}

		// POST call for setting
		if r.Method == "POST" && r.URL.Path == "/audioproductlevelcontrols" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<status>OK</status>`))

			return
		}

		t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := createTestClient(server.URL)

	err := client.SetFrontCenterSpeakerLevel(5)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
}

func TestClient_SetRearSurroundSpeakersLevel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle capabilities check
		if r.URL.Path == "/capabilities" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<capabilities><capability name="audioproductlevelcontrols"/></capabilities>`))

			return
		}

		// First call might be GET for validation
		if r.Method == "GET" && r.URL.Path == "/audioproductlevelcontrols" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<audioproductlevelcontrols>
				<frontCenterSpeakerLevel value="0" minValue="-10" maxValue="10" step="1"/>
				<rearSurroundSpeakersLevel value="0" minValue="-8" maxValue="8" step="1"/>
			</audioproductlevelcontrols>`))

			return
		}

		// POST call for setting
		if r.Method == "POST" && r.URL.Path == "/audioproductlevelcontrols" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<status>OK</status>`))

			return
		}

		t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := createTestClient(server.URL)

	err := client.SetRearSurroundSpeakersLevel(-3)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
}

func TestClient_AudioEndpoints_NetworkError(t *testing.T) {
	// Create client with invalid host to trigger network error
	config := DefaultConfig()
	config.Host = "invalid-host-that-does-not-exist"
	config.Port = 9999
	client := NewClient(config)

	// Test all audio endpoints with network errors
	_, err := client.GetAudioDSPControls()
	if err == nil {
		t.Errorf("Expected network error for GetAudioDSPControls but got none")
	}

	err = client.SetAudioDSPControls("MUSIC", 50)
	if err == nil {
		t.Errorf("Expected network error for SetAudioDSPControls but got none")
	}

	err = client.SetAudioMode("DIALOG")
	if err == nil {
		t.Errorf("Expected network error for SetAudioMode but got none")
	}

	err = client.SetVideoSyncAudioDelay(25)
	if err == nil {
		t.Errorf("Expected network error for SetVideoSyncAudioDelay but got none")
	}

	_, err = client.GetAudioProductToneControls()
	if err == nil {
		t.Errorf("Expected network error for GetAudioProductToneControls but got none")
	}

	bass := 5
	treble := -2

	err = client.SetAudioProductToneControls(&bass, &treble)
	if err == nil {
		t.Errorf("Expected network error for SetAudioProductToneControls but got none")
	}

	err = client.SetAdvancedBass(3)
	if err == nil {
		t.Errorf("Expected network error for SetAdvancedBass but got none")
	}

	err = client.SetAdvancedTreble(-1)
	if err == nil {
		t.Errorf("Expected network error for SetAdvancedTreble but got none")
	}

	_, err = client.GetAudioProductLevelControls()
	if err == nil {
		t.Errorf("Expected network error for GetAudioProductLevelControls but got none")
	}

	frontCenter := 2
	rearSurround := -1

	err = client.SetAudioProductLevelControls(&frontCenter, &rearSurround)
	if err == nil {
		t.Errorf("Expected network error for SetAudioProductLevelControls but got none")
	}

	err = client.SetFrontCenterSpeakerLevel(4)
	if err == nil {
		t.Errorf("Expected network error for SetFrontCenterSpeakerLevel but got none")
	}

	err = client.SetRearSurroundSpeakersLevel(-2)
	if err == nil {
		t.Errorf("Expected network error for SetRearSurroundSpeakersLevel but got none")
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}
