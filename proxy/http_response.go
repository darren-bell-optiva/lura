// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"compress/gzip"
	"context"
	"github.com/luraproject/lura/v2/config"
	"io"
	"net/http"
	"net/textproto"

	"github.com/luraproject/lura/v2/encoding"
)

// HTTPResponseParser defines how the response is parsed from http.Response to Response object
type HTTPResponseParser func(context.Context, *http.Response) (*Response, error)

// DefaultHTTPResponseParserConfig defines a default HTTPResponseParserConfig
var DefaultHTTPResponseParserConfig = HTTPResponseParserConfig{
	func(_ io.Reader, _ *map[string]interface{}) error { return nil },
	EntityFormatterFunc(func(r Response) Response { return r }),
}

// HTTPResponseParserConfig contains the config for a given HttpResponseParser
type HTTPResponseParserConfig struct {
	Decoder         encoding.Decoder
	EntityFormatter EntityFormatter
}

// HTTPResponseParserFactory creates HTTPResponseParser from a given HTTPResponseParserConfig
type HTTPResponseParserFactory func(HTTPResponseParserConfig) HTTPResponseParser

// DefaultHTTPResponseParserFactory is the default implementation of HTTPResponseParserFactory
func DefaultHTTPResponseParserFactory(remote *config.Backend, cfg HTTPResponseParserConfig) HTTPResponseParser {
	return func(ctx context.Context, resp *http.Response) (*Response, error) {
		defer resp.Body.Close()

		var reader io.ReadCloser
		switch resp.Header.Get("Content-Encoding") {
		case "gzip":
			reader, _ = gzip.NewReader(resp.Body)
			defer reader.Close()
		default:
			reader = resp.Body
		}

		var data map[string]interface{}
		if err := cfg.Decoder(reader, &data); err != nil {
			return nil, err
		}

		responseHeader := make(http.Header)
		for i := range remote.HeadersFromResponse {
			headerFromResponse := textproto.CanonicalMIMEHeaderKey(remote.HeadersFromResponse[i])
			if resp.Header.Get(headerFromResponse) != "" {
				responseHeader.Set(headerFromResponse, resp.Header.Get(headerFromResponse))
			}
		}

		newResponse := Response{
			Data:       data,
			IsComplete: true,
			Metadata: Metadata{
				Headers: responseHeader,
			},
		}
		newResponse = cfg.EntityFormatter.Format(newResponse)
		return &newResponse, nil
	}
}

// NoOpHTTPResponseParser is a HTTPResponseParser implementation that just copies the
// http response body into the proxy response IO
func NoOpHTTPResponseParser(ctx context.Context, resp *http.Response) (*Response, error) {
	return &Response{
		Data:       map[string]interface{}{},
		IsComplete: true,
		Io:         NewReadCloserWrapper(ctx, resp.Body),
		Metadata: Metadata{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
		},
	}, nil
}
