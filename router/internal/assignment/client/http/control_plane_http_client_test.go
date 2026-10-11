package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCurrentAssignmentSnapshot(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, httpRequest *http.Request) {
		if httpRequest.Method != http.MethodGet || httpRequest.URL.Path != "/assignments/current" {
			test.Error("unexpected request")
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"epoch":3,"partition_count":1,"nodes":[{"id":"one","url":"http://one"}],"assignments":[{"partition":0,"node_id":"one"}]}`))
	}))
	defer server.Close()
	client := &ControlPlaneHttpClient{BaseURL: server.URL, Client: server.Client()}
	snapshot, err := client.Current(context.Background())
	if err != nil || snapshot.Epoch != 3 || len(snapshot.Assignments) != 1 {
		test.Fatalf("snapshot=%+v err=%v", snapshot, err)
	}
}
