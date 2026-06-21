package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stegerj/bose-soundtouch/pkg/models"
)

func TestGetServiceAvailability(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		statusCode   int
		expectError  bool
		validate     func(t *testing.T, sa *models.ServiceAvailability)
	}{
		{
			name: "successful response with mixed availability",
			responseBody: `<?xml version="1.0" encoding="UTF-8" ?>
<serviceAvailability>
  <services>
    <service type="AIRPLAY" isAvailable="true" />
    <service type="ALEXA" isAvailable="false" />
    <service type="AMAZON" isAvailable="true" />
    <service type="BLUETOOTH" isAvailable="false" reason="INVALID_SOURCE_TYPE" />
    <service type="BMX" isAvailable="false" />
    <service type="DEEZER" isAvailable="true" />
    <service type="IHEART" isAvailable="true" />
    <service type="LOCAL_INTERNET_RADIO" isAvailable="true" />
    <service type="LOCAL_MUSIC" isAvailable="true" />
    <service type="NOTIFICATION" isAvailable="false" />
    <service type="PANDORA" isAvailable="true" />
    <service type="SPOTIFY" isAvailable="true" />
    <service type="TUNEIN" isAvailable="true" />
  </services>
</serviceAvailability>`,
			statusCode:  200,
			expectError: false,
			validate: func(t *testing.T, sa *models.ServiceAvailability) {
				t.Helper()

				if sa == nil {
					t.Fatal("service availability should not be nil")
				}

				if sa.Services == nil {
					t.Fatal("services should not be nil")
				}

				// Check total service count
				if sa.GetServiceCount() != 13 {
					t.Errorf("expected 13 services, got %d", sa.GetServiceCount())
				}

				// Check available services count
				if sa.GetAvailableServiceCount() != 9 {
					t.Errorf("expected 9 available services, got %d", sa.GetAvailableServiceCount())
				}

				// Check unavailable services count
				if sa.GetUnavailableServiceCount() != 4 {
					t.Errorf("expected 4 unavailable services, got %d", sa.GetUnavailableServiceCount())
				}

				// Check specific service availability
				if !sa.HasSpotify() {
					t.Error("should have Spotify")
				}

				if !sa.HasAirPlay() {
					t.Error("should have AirPlay")
				}

				if !sa.HasTuneIn() {
					t.Error("should have TuneIn")
				}

				if !sa.HasPandora() {
					t.Error("should have Pandora")
				}

				if !sa.HasLocalMusic() {
					t.Error("should have Local Music")
				}

				if sa.HasAlexa() {
					t.Error("should not have Alexa")
				}

				if sa.HasBluetooth() {
					t.Error("should not have Bluetooth")
				}

				// Check service with reason
				bluetoothService := sa.GetServiceByType(models.ServiceTypeBluetooth)
				if bluetoothService == nil {
					t.Fatal("bluetooth service should not be nil")
				}

				if bluetoothService.IsAvailable {
					t.Error("bluetooth service should not be available")
				}

				if bluetoothService.GetReason() != "INVALID_SOURCE_TYPE" {
					t.Errorf("expected bluetooth reason 'INVALID_SOURCE_TYPE', got '%s'", bluetoothService.GetReason())
				}

				// Check streaming services
				streamingServices := sa.GetStreamingServices()
				if len(streamingServices) != 7 {
					t.Errorf("expected 7 streaming services, got %d", len(streamingServices))
				}

				// Check local services
				localServices := sa.GetLocalServices()
				if len(localServices) != 3 {
					t.Errorf("expected 3 local services, got %d", len(localServices))
				}
			},
		},
		{
			name: "successful response with all services available",
			responseBody: `<?xml version="1.0" encoding="UTF-8" ?>
<serviceAvailability>
  <services>
    <service type="SPOTIFY" isAvailable="true" />
    <service type="BLUETOOTH" isAvailable="true" />
    <service type="AIRPLAY" isAvailable="true" />
  </services>
</serviceAvailability>`,
			statusCode:  200,
			expectError: false,
			validate: func(t *testing.T, sa *models.ServiceAvailability) {
				t.Helper()

				if sa == nil {
					t.Fatal("service availability should not be nil")
				}

				if sa.Services == nil {
					t.Fatal("services should not be nil")
				}

				if sa.GetServiceCount() != 3 {
					t.Errorf("expected 3 services, got %d", sa.GetServiceCount())
				}

				if sa.GetAvailableServiceCount() != 3 {
					t.Errorf("expected 3 available services, got %d", sa.GetAvailableServiceCount())
				}

				if sa.GetUnavailableServiceCount() != 0 {
					t.Errorf("expected 0 unavailable services, got %d", sa.GetUnavailableServiceCount())
				}

				if !sa.HasSpotify() {
					t.Error("should have Spotify")
				}

				if !sa.HasBluetooth() {
					t.Error("should have Bluetooth")
				}

				if !sa.HasAirPlay() {
					t.Error("should have AirPlay")
				}
			},
		},
		{
			name: "successful response with no services",
			responseBody: `<?xml version="1.0" encoding="UTF-8" ?>
<serviceAvailability>
  <services>
  </services>
</serviceAvailability>`,
			statusCode:  200,
			expectError: false,
			validate: func(t *testing.T, sa *models.ServiceAvailability) {
				t.Helper()

				if sa == nil {
					t.Fatal("service availability should not be nil")
				}

				if sa.Services == nil {
					t.Fatal("services should not be nil")
				}

				if sa.GetServiceCount() != 0 {
					t.Errorf("expected 0 services, got %d", sa.GetServiceCount())
				}

				if sa.GetAvailableServiceCount() != 0 {
					t.Errorf("expected 0 available services, got %d", sa.GetAvailableServiceCount())
				}

				if sa.GetUnavailableServiceCount() != 0 {
					t.Errorf("expected 0 unavailable services, got %d", sa.GetUnavailableServiceCount())
				}

				if sa.HasSpotify() {
					t.Error("should not have Spotify")
				}

				if sa.HasBluetooth() {
					t.Error("should not have Bluetooth")
				}
			},
		},
		{
			name:         "server error",
			responseBody: "Internal Server Error",
			statusCode:   500,
			expectError:  true,
			validate:     func(_ *testing.T, _ *models.ServiceAvailability) {},
		},
		{
			name:         "invalid XML",
			responseBody: "not valid xml",
			statusCode:   200,
			expectError:  true,
			validate:     func(_ *testing.T, _ *models.ServiceAvailability) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/serviceAvailability" {
					t.Errorf("expected path /serviceAvailability, got %s", r.URL.Path)
				}

				if r.Method != "GET" {
					t.Errorf("expected GET method, got %s", r.Method)
				}

				w.WriteHeader(tt.statusCode)
				_, _ = fmt.Fprint(w, tt.responseBody)
			}))
			defer server.Close()

			// Create client
			client := createTestClient(server.URL)

			// Execute test
			result, err := client.GetServiceAvailability()

			// Validate error expectation
			if tt.expectError {
				if err == nil {
					t.Error("expected an error but got none")
				}

				if result != nil {
					t.Error("expected nil result on error")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				tt.validate(t, result)
			}
		})
	}
}

func TestGetServiceAvailability_NetworkError(t *testing.T) {
	// Create client with invalid host
	client := createTestClient("http://invalid-host:99999")

	result, err := client.GetServiceAvailability()
	if err == nil {
		t.Error("expected an error but got none")
	}

	if result != nil {
		t.Error("expected nil result on error")
	}

	if err != nil && !contains(err.Error(), "failed to get service availability") {
		t.Errorf("error message should contain 'failed to get service availability', got: %v", err)
	}
}

func TestServiceAvailabilityModel_EdgeCases(t *testing.T) {
	t.Run("nil services", func(t *testing.T) {
		sa := &models.ServiceAvailability{}

		if sa.GetServiceCount() != 0 {
			t.Errorf("expected 0 service count, got %d", sa.GetServiceCount())
		}

		if sa.GetAvailableServiceCount() != 0 {
			t.Errorf("expected 0 available count, got %d", sa.GetAvailableServiceCount())
		}

		if sa.GetUnavailableServiceCount() != 0 {
			t.Errorf("expected 0 unavailable count, got %d", sa.GetUnavailableServiceCount())
		}

		if sa.HasSpotify() {
			t.Error("should not have Spotify")
		}

		if sa.GetServiceByType(models.ServiceTypeSpotify) != nil {
			t.Error("service should be nil")
		}

		if len(sa.GetAvailableServices()) != 0 {
			t.Error("available services should be empty")
		}

		if len(sa.GetUnavailableServices()) != 0 {
			t.Error("unavailable services should be empty")
		}

		if len(sa.GetStreamingServices()) != 0 {
			t.Error("streaming services should be empty")
		}

		if len(sa.GetLocalServices()) != 0 {
			t.Error("local services should be empty")
		}
	})

	t.Run("service type checking", func(t *testing.T) {
		service := models.Service{
			Type:        "SPOTIFY",
			IsAvailable: true,
		}

		if !service.IsType(models.ServiceTypeSpotify) {
			t.Error("service should be of type Spotify")
		}

		if service.IsType(models.ServiceTypeBluetooth) {
			t.Error("service should not be of type Bluetooth")
		}
	})

	t.Run("service reason handling", func(t *testing.T) {
		serviceWithReason := models.Service{
			Type:        "BLUETOOTH",
			IsAvailable: false,
			Reason:      "DEVICE_NOT_CONNECTED",
		}

		serviceWithoutReason := models.Service{
			Type:        "SPOTIFY",
			IsAvailable: true,
		}

		if serviceWithReason.GetReason() != "DEVICE_NOT_CONNECTED" {
			t.Errorf("expected DEVICE_NOT_CONNECTED, got %s", serviceWithReason.GetReason())
		}

		if serviceWithoutReason.GetReason() != "" {
			t.Errorf("expected empty reason, got %s", serviceWithoutReason.GetReason())
		}
	})
}

func BenchmarkGetServiceAvailability(b *testing.B) {
	responseBody := `<?xml version="1.0" encoding="UTF-8" ?>
<serviceAvailability>
  <services>
    <service type="SPOTIFY" isAvailable="true" />
    <service type="BLUETOOTH" isAvailable="false" reason="UNAVAILABLE" />
    <service type="AIRPLAY" isAvailable="true" />
    <service type="PANDORA" isAvailable="true" />
    <service type="TUNEIN" isAvailable="true" />
  </services>
</serviceAvailability>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, responseBody)
	}))
	defer server.Close()

	client := createTestClient(server.URL)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := client.GetServiceAvailability()
		if err != nil {
			b.Fatal(err)
		}
	}
}
