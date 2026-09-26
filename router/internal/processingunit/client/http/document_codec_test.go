package httpclient

import (
	document "autoscale-distr-storage/router/internal/document/domain"
	"encoding/json"
	"strings"
	"testing"
)

// TestOperationCodecs проверяет wire-поля исходящих команд и чтение результата всех операций.
func TestOperationCodecs(test *testing.T) {
	for _, operation := range []string{"create", "get", "update", "delete"} {
		documentCommand := document.Command{Operation: operation, Key: document.Key{PartitionKey: "a", ID: "b"}, Payload: document.Payload(`{"x":1}`), ExpectedRevision: "v1"}
		data, err := json.Marshal(documentRequest(documentCommand))
		if err != nil {
			test.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			test.Fatal(err)
		}
		_, payload := fields["payload"]
		_, revision := fields["expected_revision"]
		if payload != (operation == "create" || operation == "update") || revision != (operation == "update" || operation == "delete") {
			test.Fatalf("%s: unexpected fields %s", operation, data)
		}
		response := `{"partition_key":"a","id":"b","revision":"v2","payload":{"x":1}}`
		documentRecord, err := decodeDocumentRecord(json.NewDecoder(strings.NewReader(response)), operation)
		if err != nil || documentRecord.Key != documentCommand.Key || documentRecord.Revision != "v2" {
			test.Fatalf("%s: record=%v err=%v", operation, documentRecord, err)
		}
		if operation == "delete" && len(documentRecord.Payload) != 0 {
			test.Fatal("delete response includes payload")
		}
	}
}
