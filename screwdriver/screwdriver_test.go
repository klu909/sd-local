package screwdriver

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testDir string = "./testdata"

func validateHeader(tb testing.TB, key, value string, r *http.Request) {
	tb.Helper()
	headers, ok := r.Header[key]

	assert.True(tb, ok, fmt.Sprintf("No %s header sent in Screwdriver request", key))

	header := headers[0]
	assert.Equal(tb, value, header)
}

func TestNew(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		testToken := "token"
		testJWT := "jwt"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wantAcceptMIMEType := "application/json"
			validateHeader(t, "Accept", wantAcceptMIMEType, r)
			token := r.URL.Query().Get("api_token")
			assert.Equal(t, token, testToken)

			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")

			testBody := fmt.Sprintf(`{"token": "%s"}`, testJWT)
			fmt.Fprintln(w, testBody)
		}))

		gotAPI, err := New(server.URL, testToken)
		api, ok := gotAPI.(*sdAPI)
		assert.True(t, ok)
		assert.Nil(t, err)
		assert.Equal(t, testToken, api.UserToken)
		assert.Equal(t, server.URL, api.APIURL)
		assert.Equal(t, testJWT, api.SDJWT)
		assert.Equal(t, testJWT, api.JWT())
	})

	t.Run("failure by invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")

			testBody := fmt.Sprint(`{`)
			fmt.Fprintln(w, testBody)
		}))

		testToken := "token"
		_, err := New(server.URL, testToken)
		assert.NotNil(t, err)

		testMsg := "failed to parse JWT response:"
		assert.Contains(t, err.Error(), testMsg)

	})

	t.Run("failure by status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
		}))

		testToken := "token"
		_, err := New(server.URL, testToken)
		assert.NotNil(t, err)

		testMsg := "failed to get JWT: StatusCode 500"
		assert.Equal(t, testMsg, err.Error())
	})

	t.Run("failure by makeURL", func(t *testing.T) {
		testURL := "http://example.com:yyy"
		testToken := "token"
		_, err := New(testURL, testToken)
		msg := err.Error()
		assert.Equal(t, 0, strings.Index(msg, "failed to make request url: "), fmt.Sprintf("expected error is `failed to make request url: ...`, actual: `%v`", msg))
	})

	t.Run("failure by sd.request", func(t *testing.T) {
		testURL := "http://localhost"
		testToken := "testToken"
		_, err := New(testURL, testToken)
		msg := err.Error()
		assert.Equal(t, 0, strings.Index(msg, "failed to send request: "), fmt.Sprintf("expected error is `failed to send request: ...`, actual: `%v`", msg))
	})
}

func TestJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wantContentType := "application/json"
			validateHeader(t, "Content-Type", wantContentType, r)
			wantAuthBearer := "Bearer jwt"
			validateHeader(t, "Authorization", wantAuthBearer, r)

			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")

			testJSON, err := ioutil.ReadFile(path.Join(testDir, "validatedSuccess.json"))
			assert.Nil(t, err)
			fmt.Fprintln(w, string(testJSON))
		}))

		testAPI := sdAPI{
			HTTPClient: http.DefaultClient,
			UserToken:  "dummy",
			APIURL:     server.URL,
			SDJWT:      "jwt",
		}

		testJob := Job{
			Steps: []Step{
				{
					Name:    "install",
					Command: "echo install",
				},
				{
					Name:    "publish",
					Command: "echo publish",
				},
			},
			Environment: map[string]string{
				"TEST_ENV": "hoge",
			},
			Image: "alpine",
		}

		gotJob, err := testAPI.Job("main", path.Join(testDir, "screwdriver.yaml"))
		assert.Nil(t, err)
		assert.Equal(t, testJob, gotJob)
	})

	t.Run("failure by making URL", func(t *testing.T) {
		testAPI := sdAPI{
			HTTPClient: http.DefaultClient,
			UserToken:  "dummy",
			APIURL:     "http://example.com:yyy",
			SDJWT:      "jwt",
		}

		_, err := testAPI.Job("main", path.Join(testDir, "screwdriver.yaml"))
		assert.NotNil(t, err)

		msg := err.Error()
		assert.Equal(t, 0, strings.Index(msg, "failed to make request url: "), fmt.Sprintf("expected error is `failed to create api endpoint URL: ...`, actual: `%v`", msg))
	})

	t.Run("failure by invalid JSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")

			testBody := fmt.Sprint(`{`)
			fmt.Fprintln(w, testBody)
		}))

		testAPI := sdAPI{
			HTTPClient: http.DefaultClient,
			UserToken:  "dummy",
			APIURL:     server.URL,
			SDJWT:      "jwt",
		}

		_, err := testAPI.Job("main", path.Join(testDir, "screwdriver.yaml"))
		assert.NotNil(t, err)

		msg := err.Error()
		assert.Equal(t, 0, strings.Index(msg, "failed to parse validator response: "), fmt.Sprintf("expected error is `failed to parse validator response: ...`, actual: `%v`", msg))
	})

	t.Run("failure by reading screwdriver.yaml", func(t *testing.T) {
		testAPI := sdAPI{
			HTTPClient: http.DefaultClient,
			UserToken:  "dummy",
			APIURL:     "http://example.com",
			SDJWT:      "jwt",
		}

		_, err := testAPI.Job("main", "./not-exist")
		assert.NotNil(t, err)

		msg := err.Error()
		assert.Equal(t, 0, strings.Index(msg, "failed to read screwdriver.yaml: "), fmt.Sprintf("expected error is `failed to read screwdriver.yaml: ...`, actual: `%v`", msg))
	})

	t.Run("failure by sending request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(301)
			w.Header().Set("Location", "")
		}))

		testAPI := sdAPI{
			HTTPClient: http.DefaultClient,
			UserToken:  "dummy",
			APIURL:     server.URL,
			SDJWT:      "jwt",
		}

		_, err := testAPI.Job("main", path.Join(testDir, "screwdriver.yaml"))
		assert.NotNil(t, err)

		msg := err.Error()
		assert.Equal(t, 0, strings.Index(msg, "failed to send request: "), fmt.Sprintf("expected error is `failed to send request: ...`, actual: `%v`", msg))
	})

	t.Run("failure by status", func(t *testing.T) {

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
		}))

		testAPI := sdAPI{
			HTTPClient: http.DefaultClient,
			UserToken:  "dummy",
			APIURL:     server.URL,
			SDJWT:      "jwt",
		}

		_, err := testAPI.Job("main", path.Join(testDir, "screwdriver.yaml"))

		assert.NotNil(t, err)

		msg := err.Error()
		assert.Equal(t, 0, strings.Index(msg, "failed to post validator: "), fmt.Sprintf("expected error is `failed to post validator: ...`, actual: `%v`", msg))
	})

	t.Run("failure by invalid screwdriver.yaml", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")

			testJSON, err := ioutil.ReadFile(path.Join(testDir, "validatedFailed.json"))
			assert.Nil(t, err)

			fmt.Fprintln(w, string(testJSON))
		}))

		testAPI := sdAPI{
			HTTPClient: http.DefaultClient,
			UserToken:  "dummy",
			APIURL:     server.URL,
			SDJWT:      "jwt",
		}

		_, err := testAPI.Job("main", path.Join(testDir, "screwdriver.yaml"))
		assert.NotNil(t, err)

		msg := err.Error()
		assert.Equal(t, 0, strings.Index(msg, "failed to parse screwdriver.yaml: "), fmt.Sprintf("expected error is `failed to parse screwdriver.yaml: ...`, actual: `%v`", msg))
	})

	t.Run("failure by not found job name in parsed screwdriver.yaml", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")

			testJSON, err := ioutil.ReadFile(path.Join(testDir, "validatedSuccess.json"))
			assert.Nil(t, err)
			fmt.Fprintln(w, string(testJSON))
		}))

		testAPI := sdAPI{
			HTTPClient: http.DefaultClient,
			UserToken:  "dummy",
			APIURL:     server.URL,
			SDJWT:      "jwt",
		}

		_, err := testAPI.Job("nyancat", path.Join(testDir, "screwdriver.yaml"))
		assert.NotNil(t, err)
		msg := err.Error()
		assert.Equal(t, "not found 'nyancat' in parsed screwdriver.yaml", msg)
	})
}