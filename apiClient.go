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
	Buffer  *bytes.Buffer
	Encoder *json.Encoder
}

func (encoder *pooledEncoder) Encode(value any) error {
	return encoder.Encoder.Encode(value)
}

func (encoder *pooledEncoder) Reset() {
	encoder.Buffer.Reset()
}

var encoderPool = sync.Pool{
	New: func() any {
		buffer := new(bytes.Buffer)

		return &pooledEncoder{
			Buffer:  buffer,
			Encoder: json.NewEncoder(buffer),
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
			encoder.Reset()
			encoderPool.Put(encoder)
		}()

		if err := encoder.Encode(params.Body); err != nil {
			return nil, err
		}

		request.Body = io.NopCloser(encoder.Buffer)
		request.ContentLength = int64(encoder.Buffer.Len())
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
