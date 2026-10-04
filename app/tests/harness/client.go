package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"strings"
)

type Client struct {
	baseURL string
	http    *http.Client
	tokens  map[string]string
}

func NewClient(baseURL string) *Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Jar: jar,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		tokens: make(map[string]string),
	}
}

func (c *Client) SetToken(identity, token string) {
	c.tokens[identity] = token
}

func (c *Client) Login(identity, email, password string) error {
	status, _, body, err := c.doRaw(RequestSpec{
		Method: http.MethodGet,
		Path:   "/api/auth/login",
		Query: map[string][]string{
			"email": {email},
			"pwd":   {password},
		},
	})
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("login status = %d", status)
	}
	var response struct {
		OK    bool   `json:"Ok"`
		Token string `json:"Token"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}
	if !response.OK || response.Token == "" {
		return fmt.Errorf("login did not return a token: %s", body)
	}
	c.SetToken(identity, response.Token)
	return nil
}

func (c *Client) Do(spec RequestSpec) (Snapshot, error) {
	status, headers, body, err := c.doRaw(spec)
	if err != nil {
		return Snapshot{}, err
	}
	normalizedHeaders, err := NormalizeHeaders(headers, spec.Binary)
	if err != nil {
		return Snapshot{}, err
	}
	if !spec.Binary && isJSON(normalizedHeaders["Content-Type"]) {
		body, err = NormalizeBody(body)
		if err != nil {
			return Snapshot{}, err
		}
	}
	return Snapshot{Status: status, Headers: normalizedHeaders, Body: body, normalized: true}, nil
}

func (c *Client) doRaw(spec RequestSpec) (int, http.Header, []byte, error) {
	if spec.Method == "" || spec.Path == "" {
		return 0, nil, nil, fmt.Errorf("request method and path are required")
	}
	requestURL, err := url.Parse(c.baseURL + spec.Path)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("parse request URL: %w", err)
	}
	query := make(url.Values, len(spec.Query)+1)
	for key, values := range spec.Query {
		query[key] = append([]string(nil), values...)
	}
	switch spec.Auth {
	case "", "none":
	case "invalid":
		query.Set("token", "not-a-valid-token")
	default:
		token := c.tokens[spec.Auth]
		if token == "" {
			return 0, nil, nil, fmt.Errorf("no token for identity %q", spec.Auth)
		}
		query.Set("token", token)
	}
	requestURL.RawQuery = query.Encode()

	var body *bytes.Reader
	var contentType string
	if len(spec.Files) > 0 {
		multipartBody, multipartContentType, err := encodeMultipartForm(spec.Form, spec.Files)
		if err != nil {
			return 0, nil, nil, err
		}
		body = bytes.NewReader(multipartBody)
		contentType = multipartContentType
	} else if len(spec.Form) > 0 {
		body = bytes.NewReader([]byte(url.Values(spec.Form).Encode()))
		contentType = "application/x-www-form-urlencoded"
	} else {
		body = bytes.NewReader(nil)
	}
	request, err := http.NewRequest(spec.Method, requestURL.String(), body)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("create request: %w", err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("execute %s %s: %w", spec.Method, spec.Path, err)
	}
	defer response.Body.Close()
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("read response body: %w", err)
	}
	return response.StatusCode, response.Header, responseBody, nil
}

func encodeMultipartForm(form map[string][]string, files map[string]FilePart) ([]byte, string, error) {
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)
	for key, values := range form {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return nil, "", fmt.Errorf("write multipart field %q: %w", key, err)
			}
		}
	}
	for key, file := range files {
		contentType := file.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		headers := textproto.MIMEHeader{}
		headers.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
			"name":     key,
			"filename": file.Filename,
		}))
		headers.Set("Content-Type", contentType)
		part, err := writer.CreatePart(headers)
		if err != nil {
			return nil, "", fmt.Errorf("create multipart file %q: %w", key, err)
		}
		if _, err := part.Write(file.Body); err != nil {
			return nil, "", fmt.Errorf("write multipart file %q: %w", key, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("finish multipart form: %w", err)
	}
	return buffer.Bytes(), writer.FormDataContentType(), nil
}
