package collect

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
)

type Headers struct {
	ContentLength string `json:"Content-Length"`
	ContentType   string `json:"Content-Type"`
	Date          string `json:"Date,omitempty"`
}

type Response struct {
	Status  int     `json:"status"`
	Body    string  `json:"body"`
	Headers Headers `json:"headers"`
}

type ResponseData struct {
	Response Response `json:"response"`
}

type args struct {
	progressChan chan<- interface{}
}

type CollectorTest struct {
	name         string
	httpServer   *http.Server
	isHttps      bool
	Collector    *troubleshootv1beta2.HTTP
	args         args
	checkTimeout bool
	want         CollectorResult
	wantErr      bool
}

type ErrorResponse struct {
	Error HTTPError `json:"error"`
}

func (r ResponseData) ToJSONbytes() ([]byte, error) {
	return json.Marshal(r)
}

func (r ErrorResponse) ToJSONbytes() ([]byte, error) {
	return json.Marshal(r)
}

func TestCollectHTTP_Collect(t *testing.T) {

	mux := http.NewServeMux()
	mux.HandleFunc("/get", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json; charset=utf-8")
		res.WriteHeader(http.StatusOK)
		res.Write([]byte("{\"status\": \"healthy\"}"))
	})
	mux.HandleFunc("/post", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "text/plain; charset=utf-8")
		res.WriteHeader(http.StatusOK)
		res.Write([]byte("Hello, POST!"))
	})
	mux.HandleFunc("/put", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "text/plain; charset=utf-8")
		res.WriteHeader(http.StatusOK)
		res.Write([]byte("Hello, PUT!"))
	})
	mux.HandleFunc("/error", func(res http.ResponseWriter, req *http.Request) {
		time.Sleep(1 * time.Millisecond)
		fmt.Println("Sleeping for 2 seconds on /error call")
		res.Header().Set("Content-Type", "application/json; charset=utf-8")
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte("{\"error\": { \"message\": \"context deadline exceeded\"}}"))
	})

	sample_get_response := &ResponseData{
		Response: Response{
			Status: 200,
			Body:   "{\"status\": \"healthy\"}",
			Headers: Headers{
				ContentLength: "21",
				ContentType:   "application/json; charset=utf-8",
			},
		},
	}
	sample_get_bytes, _ := sample_get_response.ToJSONbytes()

	sample_post_response := &ResponseData{
		Response: Response{
			Status: 200,
			Body:   "Hello, POST!",
			Headers: Headers{
				ContentLength: "12",
				ContentType:   "text/plain; charset=utf-8",
			},
		},
	}
	sample_post_bytes, _ := sample_post_response.ToJSONbytes()

	sample_put_response := &ResponseData{
		Response: Response{
			Status: 200,
			Body:   "Hello, PUT!",
			Headers: Headers{
				ContentLength: "11",
				ContentType:   "text/plain; charset=utf-8",
			},
		},
	}
	sample_put_bytes, _ := sample_put_response.ToJSONbytes()

	sample_error_response := &ErrorResponse{
		Error: HTTPError{
			Message: "context deadline exceeded",
		},
	}

	sample_error_bytes, _ := sample_error_response.ToJSONbytes()

	tests := []CollectorTest{
		{
			// check valid file path when CollectorName is not supplied
			name: "GET: collector name unset",
			Collector: &troubleshootv1beta2.HTTP{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{
					CollectorName: "",
				},
				Get: &troubleshootv1beta2.Get{},
			},
			args: args{
				progressChan: nil,
			},
			want: CollectorResult{
				"result.json": sample_get_bytes,
			},
			checkTimeout: false,
			wantErr:      false,
			isHttps:      false,
		},
		{
			// check valid file path when CollectorName is supplied
			name: "GET: valid collect",
			Collector: &troubleshootv1beta2.HTTP{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{
					CollectorName: "example-com",
				},
				Get: &troubleshootv1beta2.Get{},
			},
			args: args{
				progressChan: nil,
			},
			want: CollectorResult{
				"example-com.json": sample_get_bytes,
			},
			checkTimeout: false,
			wantErr:      false,
			isHttps:      false,
		},
		{
			// check valid file path when CollectorName is supplied
			name: "POST: valid collect",
			Collector: &troubleshootv1beta2.HTTP{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{
					CollectorName: "example-com",
				},
				Post: &troubleshootv1beta2.Post{
					InsecureSkipVerify: true,
					Body:               `{"id": 123, "name": "John Doe"}`,
					Headers:            map[string]string{"Content-Type": "application/json"},
				},
			},
			args: args{
				progressChan: nil,
			},
			want: CollectorResult{
				"example-com.json": sample_post_bytes,
			},
			checkTimeout: false,
			wantErr:      false,
			isHttps:      false,
		},
		{
			// check valid file path when CollectorName is supplied
			name: "PUT: valid collect",
			Collector: &troubleshootv1beta2.HTTP{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{
					CollectorName: "example-com",
				},
				Put: &troubleshootv1beta2.Put{
					Body:    `{"id": 123, "name": "John Doe"}`,
					Headers: map[string]string{"Content-Type": "application/json"},
				},
			},
			args: args{
				progressChan: nil,
			},
			want: CollectorResult{
				"example-com.json": sample_put_bytes,
			},
			checkTimeout: false,
			wantErr:      false,
			isHttps:      false,
		},
		{
			name: "ERROR: check request timeout < server delay (exit early)",
			Collector: &troubleshootv1beta2.HTTP{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{
					CollectorName: "example-com",
				},
				Get: &troubleshootv1beta2.Get{
					Timeout: "200ns",
				},
			},
			args: args{
				progressChan: nil,
			},
			want: CollectorResult{
				"example-com.json": sample_error_bytes,
			},
			checkTimeout: true,
			wantErr:      true,
		},
		{
			name: "GET: check request timeout > server delay",
			Collector: &troubleshootv1beta2.HTTP{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{
					CollectorName: "",
				},
				Get: &troubleshootv1beta2.Get{
					Timeout: "300ms",
				},
			},
			args: args{
				progressChan: nil,
			},
			want: CollectorResult{
				"result.json": sample_get_bytes,
			},
			checkTimeout: true,
			wantErr:      false,
		},
		// TODO: add TLS cert case
	}
	for _, tt := range tests {
		var ts *httptest.Server
		if tt.isHttps {
			ts = httptest.NewTLSServer(mux)
		} else {
			ts = httptest.NewServer(mux)
		}
		url := ts.URL
		defer ts.Close()

		t.Run(tt.name, func(t *testing.T) {
			c := &CollectHTTP{
				Collector: tt.Collector,
			}

			switch {
			case c.Collector.Get != nil:
				if tt.checkTimeout && tt.wantErr {
					c.Collector.Get.URL = fmt.Sprintf("%s%s", url, "/error")
					response_data := sample_error_response
					response_data.testCollectHTTP(t, &tt, c)
				} else {
					c.Collector.Get.URL = fmt.Sprintf("%s%s", url, "/get")
					response_data := sample_get_response
					response_data.testCollectHTTP(t, &tt, c)
				}
			case c.Collector.Post != nil:
				c.Collector.Post.URL = fmt.Sprintf("%s%s", url, "/post")
				response_data := sample_post_response
				response_data.testCollectHTTP(t, &tt, c)
			case c.Collector.Put != nil:
				c.Collector.Put.URL = fmt.Sprintf("%s%s", url, "/put")
				response_data := sample_put_response
				response_data.testCollectHTTP(t, &tt, c)
			default:
				t.Errorf("ERR: Method not supported")
			}
		})
	}
}

func (rd *ResponseData) testCollectHTTP(t *testing.T, tt *CollectorTest, c *CollectHTTP) {

	got, err := c.Collect(tt.args.progressChan)
	if (err != nil) != tt.wantErr {
		t.Errorf("CollectHTTP.Collect() error = %v, wantErr %v", err, tt.wantErr)
		return
	}
	var expected_filename string
	if c.Collector.CollectorName == "" {
		expected_filename = "result.json"
	} else {
		expected_filename = c.Collector.CollectorName + ".json"
	}

	var resp ResponseData
	if err := json.Unmarshal(got[expected_filename], &resp); err != nil {
		t.Errorf("CollectHTTP.Collect() error = %v, wantErr %v", err, tt.wantErr)
		return
	}

	// Correct format of the collected data (JSON data)
	assert.Equal(t, rd.Response.Status, resp.Response.Status)
	assert.Equal(t, rd.Response.Body, resp.Response.Body)
	assert.Equal(t, rd.Response.Headers.ContentLength, resp.Response.Headers.ContentLength)
}

func (er *ErrorResponse) testCollectHTTP(t *testing.T, tt *CollectorTest, c *CollectHTTP) {

	got, err := c.Collect(tt.args.progressChan)
	if err != nil {
		t.Errorf("CollectHTTP.Collect() error = %v, wantErr %v", err, tt.wantErr)
		return
	}
	var expected_filename string
	if c.Collector.CollectorName == "" {
		expected_filename = "result.json"
	} else {
		expected_filename = c.Collector.CollectorName + ".json"
	}

	// assert er.Error.Message is not nil
	assert.NotNil(t, er.Error.Message)

	if err := json.Unmarshal(got[expected_filename], &er); err != nil {
		t.Errorf("CollectHTTP.Collect() error = %v, wantErr %v", err, tt.wantErr)
		return
	}

	if strings.Contains(strings.TrimSpace(er.Error.Message), "context deadline exceeded") != tt.wantErr {
		t.Errorf("CollectHTTP.Collect() response = %v, wantErr %v", er.Error.Message, tt.wantErr)
	}
}

func Test_parseTimeout(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:  "1s timeout",
			input: "1s",
			want:  time.Second,
		},
		{
			name:  "empty timeout",
			input: "",
			want:  0,
		},
		{
			name:    "negative timeout",
			input:   "-1s",
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimeout(tt.input)
			assert.Equal(t, (err != nil), tt.wantErr)
			assert.Equal(t, got, tt.want)
		})
	}
}
