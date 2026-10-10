package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
)

type pooledEncoder struct {
	buffer  *bytes.Buffer
	encoder *json.Encoder
}

func (encoder *pooledEncoder) encode(value any) error {
	return encoder.encoder.Encode(value)
}

func (encoder *pooledEncoder) reset() {
	encoder.buffer.Reset()
}

var encoderPool = sync.Pool{
	New: func() any {
		buffer := new(bytes.Buffer)

		return &pooledEncoder{
			buffer:  buffer,
			encoder: json.NewEncoder(buffer),
		}
	},
}

type ApiClient struct {
	baseUrl string
}

type ApiClientRequestParameters struct {
	Method string
	Path   string
	Body   any
	Header http.Header
}

func NewApiClient(baseUrl string) *ApiClient {
	return &ApiClient{
		baseUrl: strings.TrimRight(baseUrl, "/"),
	}
}

func (client *ApiClient) SendRequest[TRequest any, TResponse any](
	context context.Context,
	params ApiClientRequestParameters) (*TResponse, error) {

	fullUrl := client.baseUrl + "/" + strings.TrimLeft(params.Path, "/")
	request, err := http.NewRequestWithContext(context, params.Method, fullUrl, nil)
	if err != nil {
		return nil, err
	}

	if params.Body != nil {
		var encoder = encoderPool.Get().(*pooledEncoder)
		defer func() {
			encoder.reset()
			encoderPool.Put(encoder)
		}()

		if err := encoder.encode(params.Body); err != nil {
			return nil, err
		}

		request.Body = io.NopCloser(encoder.buffer)
		request.ContentLength = int64(encoder.buffer.Len())
	}

	if params.Header != nil {
		request.Header = params.Header
	}

	httpResponse, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer httpResponse.Body.Close()

	bodyBytes, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}

	var response TResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, err
	}

	return &response, nil
}
