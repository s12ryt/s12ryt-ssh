package remote

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var ErrInvalidRemotePath = errors.New("remote: resource ID and object key are required")

// ListObjects returns metadata from one assigned S3 connection.
func (s *Session) ListObjects(ctx context.Context, resourceID, prefix string) ([]S3Object, error) {
	path, err := resourcePath(resourceID, "/s3/objects")
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("prefix", prefix)
	path += "?" + query.Encode()
	var response struct {
		Objects []S3Object `json:"objects"`
	}
	if err := s.authorizedJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Objects, nil
}

// UploadObject streams an object through the proxy. Body must support rewind for one token retry.
func (s *Session) UploadObject(ctx context.Context, resourceID, key string, body io.ReadSeeker, contentLength int64) (UploadResult, error) {
	path, err := objectPath(resourceID, "/s3/objects/", key)
	if err != nil {
		return UploadResult{}, err
	}
	if body == nil {
		return UploadResult{}, errors.New("remote: upload body is required")
	}
	var result UploadResult
	err = s.authorized(ctx, func(accessToken string) error {
		if _, err := body.Seek(0, io.SeekStart); err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPut, s.client.endpoint(path), body)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+accessToken)
		request.Header.Set("Content-Type", "application/octet-stream")
		if contentLength >= 0 {
			request.ContentLength = contentLength
		}
		return s.client.doRequest(request, &result)
	})
	return result, err
}

// DownloadObject opens a streaming object response from the proxy.
func (s *Session) DownloadObject(ctx context.Context, resourceID, key string) (Download, error) {
	path, err := objectPath(resourceID, "/s3/download/", key)
	if err != nil {
		return Download{}, err
	}
	var download Download
	err = s.authorized(ctx, func(accessToken string) error {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.client.endpoint(path), nil)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+accessToken)
		response, err := s.client.http.Do(request)
		if err != nil {
			return err
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			defer response.Body.Close()
			return decodeAPIError(response)
		}
		download = Download{
			Body:          response.Body,
			ContentLength: response.ContentLength,
			ContentType:   response.Header.Get("Content-Type"),
		}
		return nil
	})
	return download, err
}

// DeleteObject removes one object through an assigned S3 connection.
func (s *Session) DeleteObject(ctx context.Context, resourceID, key string) error {
	path, err := objectPath(resourceID, "/s3/objects/", key)
	if err != nil {
		return err
	}
	return s.authorizedJSON(ctx, http.MethodDelete, path, nil, nil)
}

// Tables lists tables visible through an assigned SQL connection.
func (s *Session) Tables(ctx context.Context, resourceID string) ([]string, error) {
	path, err := resourcePath(resourceID, "/sql/tables")
	if err != nil {
		return nil, err
	}
	var response struct {
		Tables []string `json:"tables"`
	}
	if err := s.authorizedJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Tables, nil
}

// Query executes a server-enforced read-only SQL query.
func (s *Session) Query(ctx context.Context, resourceID, statement string, parameters []any) (SQLQueryResult, error) {
	path, err := resourcePath(resourceID, "/sql/query")
	if err != nil {
		return SQLQueryResult{}, err
	}
	var result SQLQueryResult
	err = s.authorizedJSON(ctx, http.MethodPost, path, sqlRequestBody{Statement: statement, Parameters: parameters}, &result)
	return result, err
}

// Exec executes SQL with side effects when the account has sql.exec permission.
func (s *Session) Exec(ctx context.Context, resourceID, statement string, parameters []any) (SQLExecResult, error) {
	path, err := resourcePath(resourceID, "/sql/exec")
	if err != nil {
		return SQLExecResult{}, err
	}
	var result SQLExecResult
	err = s.authorizedJSON(ctx, http.MethodPost, path, sqlRequestBody{Statement: statement, Parameters: parameters}, &result)
	return result, err
}

type sqlRequestBody struct {
	Statement  string `json:"statement"`
	Parameters []any  `json:"parameters,omitempty"`
}

func resourcePath(resourceID, suffix string) (string, error) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" || strings.ContainsAny(resourceID, "/\\") {
		return "", ErrInvalidRemotePath
	}
	return "/api/v1/resources/" + resourceID + suffix, nil
}

func objectPath(resourceID, prefix, key string) (string, error) {
	path, err := resourcePath(resourceID, prefix)
	if err != nil {
		return "", err
	}
	key = strings.TrimLeft(strings.ReplaceAll(key, "\\", "/"), "/")
	if key == "" {
		return "", ErrInvalidRemotePath
	}
	segments := strings.Split(key, "/")
	for index, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", ErrInvalidRemotePath
		}
		segments[index] = segment
	}
	return path + strings.Join(segments, "/"), nil
}
