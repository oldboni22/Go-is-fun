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
		var pooled = encoderPool.Get().(*pooledEncoder)
		defer func() {
			pooled.Buffer.Reset()
			encoderPool.Put(pooled)
		}()

		if err := pooled.Encoder.Encode(params.Body); err != nil {
			return nil, err
		}

		request.Body = io.NopCloser(pooled.Buffer)
		request.ContentLength = int64(pooled.Buffer.Len())
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
