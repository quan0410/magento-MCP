package magento

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGraphQLClient_QuerySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			t.Errorf("expected /graphql, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"products": {
					"total_count": 1,
					"items": [
						{"sku": "TEST-SKU", "name": "Test Blazer"}
					]
				}
			}
		}`))
	}))
	defer srv.Close()

	client, err := NewGraphQLClient(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	data, err := client.Query(context.Background(), "query { products { items { sku name } } }", nil)
	if err != nil {
		t.Fatalf("unexpected error executing query: %v", err)
	}

	var resp struct {
		Products struct {
			TotalCount int `json:"total_count"`
			Items      []struct {
				SKU  string `json:"sku"`
				Name string `json:"name"`
			} `json:"items"`
		} `json:"products"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Products.TotalCount != 1 || len(resp.Products.Items) != 1 || resp.Products.Items[0].SKU != "TEST-SKU" {
		t.Errorf("unexpected parsed response: %+v", resp)
	}
}

func TestGraphQLClient_GraphQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"errors": [
				{"message": "Field 'invalid' doesn't exist on type 'Query'"}
			]
		}`))
	}))
	defer srv.Close()

	client, err := NewGraphQLClient(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.Query(context.Background(), "query { invalid }", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
