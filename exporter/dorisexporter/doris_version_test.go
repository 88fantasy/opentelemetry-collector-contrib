// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

func TestDetectDorisVersion(t *testing.T) {
	tests := []struct {
		name              string
		backendsResponse  string
		versionResponse   string
		wantVersion       int
		wantDetectError   bool
		wantNonFiniteJSON bool
	}{
		{
			name:              "all BEs support non-finite JSON",
			backendsResponse:  `{"data":{"backends":[{"ip":"127.0.0.1","http_port":0}]}}`,
			versionResponse:   `{"data":{"beVersionInfo":{"dorisBuildVersionMajor":4}}}`,
			wantVersion:       4,
			wantNonFiniteJSON: true,
		},
		{
			name:             "old BE",
			backendsResponse: `{"data":{"backends":[{"ip":"127.0.0.1","http_port":0}]}}`,
			versionResponse:  `{"data":{"beVersionInfo":{"dorisBuildVersionMajor":3}}}`,
			wantVersion:      3,
		},
		{
			name:             "unknown",
			backendsResponse: `{"data":{"backends":[]}}`,
			wantDetectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/backends" {
					response := tt.backendsResponse
					if strings.Contains(response, `"http_port":0`) {
						port := server.Listener.Addr().(*net.TCPAddr).Port
						response = fmt.Sprintf(`{"data":{"backends":[{"ip":"127.0.0.1","http_port":%d}]}}`, port)
					}
					_, _ = w.Write([]byte(response))
					return
				}
				_, _ = w.Write([]byte(tt.versionResponse))
			}))
			defer server.Close()

			cfg := createDefaultConfig().(*Config)
			cfg.ClientConfig.Endpoint = server.URL
			version, err := detectDorisVersion(t.Context(), server.Client(), cfg)
			if tt.wantDetectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantVersion, version)
			require.Equal(t, tt.wantNonFiniteJSON, version >= 4)
		})
	}
}

func TestMetricsExporterStartDetectsDorisVersion(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/backends" {
			port := server.Listener.Addr().(*net.TCPAddr).Port
			_, _ = fmt.Fprintf(w, `{"data":{"backends":[{"ip":"127.0.0.1","http_port":%d}]}}`, port)
			return
		}
		if r.URL.Path != "/api/be_version_info" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"beVersionInfo":{"dorisBuildVersionMajor":4}}}`))
	}))
	defer server.Close()

	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig.Endpoint = server.URL
	cfg.CreateSchema = false
	cfg.LogProgressInterval = 0
	require.NoError(t, cfg.Validate())
	exporter := newMetricsExporter(zap.NewNop(), cfg, componenttest.NewNopTelemetrySettings())
	require.NoError(t, exporter.start(t.Context(), componenttest.NewNopHost()))
	require.True(t, exporter.allowNonFinite)
	require.NoError(t, exporter.shutdown(t.Context()))
}
