package pluginproxy

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/util"
)

func applyRoute(ctx context.Context, req *http.Request, proxyPath string, route *plugins.AppPluginRoute, ds *m.DataSource) {
	proxyPath = strings.TrimPrefix(proxyPath, route.Path)

	data := templateData{
		JsonData:       ds.JsonData.Interface().(map[string]interface{}),
		SecureJsonData: ds.SecureJsonData.Decrypt(),
	}

	interpolatedURL, err := interpolateString(route.Url, data)
	if err != nil {
		logger.Error("Error interpolating proxy url", "error", err)
		return
	}

	routeURL, err := url.Parse(interpolatedURL)
	if err != nil {
		logger.Error("Error parsing plugin route url", "error", err)
		return
	}

	req.URL.Scheme = routeURL.Scheme
	req.URL.Host = routeURL.Host
	req.Host = routeURL.Host
	req.URL.Path = util.JoinUrlFragments(routeURL.Path, proxyPath)

	if err := addHeaders(&req.Header, route, data); err != nil {
		logger.Error("Failed to render plugin headers", "error", err)
	}

	tokenProvider := newAccessTokenProvider(ds.Id, route)

	if route.TokenAuth != nil {
		if token, err := tokenProvider.getAccessToken(data); err != nil {
			logger.Error("Failed to get access token", "error", err)
		} else {
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		}
	}

	if route.JwtTokenAuth != nil {
		if token, err := tokenProvider.getJwtAccessToken(ctx, data); err != nil {
			logger.Error("Failed to get access token", "error", err)
		} else {
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		}
	}
	logger.Info("Requesting", "url", req.URL.String())

}

func interpolateString(text string, data templateData) (string, error) {
	t, err := template.New("content").Parse(text)
	if err != nil {
		return "", fmt.Errorf("could not parse template %s", text)
	}

	var contentBuf bytes.Buffer
	err = t.Execute(&contentBuf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template %s", text)
	}

	return contentBuf.String(), nil
}

func addHeaders(reqHeaders *http.Header, route *plugins.AppPluginRoute, data templateData) error {
	for _, header := range route.Headers {
		interpolated, err := interpolateString(header.Content, data)
		if err != nil {
			return err
		}
		reqHeaders.Add(header.Name, interpolated)
	}

	return nil
}