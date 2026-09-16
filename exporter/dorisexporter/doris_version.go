// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type dorisBackend struct {
	IP       string `json:"ip"`
	HTTPPort int    `json:"http_port"`
}

type dorisBackendsResponse struct {
	Data struct {
		Backends []dorisBackend `json:"backends"`
	} `json:"data"`
}

type dorisVersionResponse struct {
	Data struct {
		BEVersionInfo *dorisVersionInfo `json:"beVersionInfo"`
		FEVersionInfo *dorisVersionInfo `json:"feVersionInfo"`
	} `json:"data"`
}

type dorisVersionInfo struct {
	Major *int `json:"dorisBuildVersionMajor"`
}

func detectDorisVersion(ctx context.Context, client *http.Client, cfg *Config) (int, error) {
	backends, err := requestDorisBackends(ctx, client, cfg)
	if err != nil {
		return 0, err
	}
	if len(backends) == 0 {
		return 0, errors.New("no alive Doris backends found")
	}
	baseURL, err := url.Parse(cfg.ClientConfig.Endpoint)
	if err != nil || baseURL.Scheme == "" {
		return 0, errors.New("Doris endpoint scheme is missing")
	}

	for _, backend := range backends {
		if backend.IP == "" || backend.HTTPPort <= 0 {
			return 0, errors.New("Doris backend address is missing")
		}
		endpoint := baseURL.Scheme + "://" + net.JoinHostPort(backend.IP, strconv.Itoa(backend.HTTPPort)) + "/api/be_version_info"
		version, err := requestDorisVersion(ctx, client, cfg, endpoint)
		if err != nil {
			return 0, fmt.Errorf("unable to determine Doris version for backend %s: %w", endpoint, err)
		}
		if version < 4 {
			return version, nil
		}
	}
	return 4, nil
}

func requestDorisBackends(ctx context.Context, client *http.Client, cfg *Config) ([]dorisBackend, error) {
	var response dorisBackendsResponse
	endpoint := strings.TrimRight(cfg.ClientConfig.Endpoint, "/") + "/api/backends?is_alive=true"
	if err := requestDorisJSON(ctx, client, cfg, endpoint, &response); err != nil {
		return nil, err
	}
	return response.Data.Backends, nil
}

func requestDorisVersion(ctx context.Context, client *http.Client, cfg *Config, endpoint string) (int, error) {
	var response dorisVersionResponse
	if err := requestDorisJSON(ctx, client, cfg, endpoint, &response); err != nil {
		return 0, err
	}

	info := response.Data.BEVersionInfo
	if info == nil {
		info = response.Data.FEVersionInfo
	}
	if info == nil || info.Major == nil {
		return 0, errors.New("version fields are missing")
	}
	return *info.Major, nil
}

func requestDorisJSON(ctx context.Context, client *http.Client, cfg *Config, endpoint string, response any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return err
	}
	req.SetBasicAuth(cfg.Username, string(cfg.Password))
	req.Header.Set("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected HTTP status: %s", res.Status)
	}

	return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(response)
}
