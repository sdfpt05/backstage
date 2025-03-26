package elasticsearch

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	
	"example.com/backstage/services/truck/config"
	
	"github.com/elastic/go-elasticsearch/v8"
)

// Client is an interface for Elasticsearch operations
type Client interface {
	IndexDocument(ctx context.Context, id string, document []byte) error
	SearchDocuments(ctx context.Context, query interface{}) ([]json.RawMessage, error)
	GetDocument(ctx context.Context, id string) (json.RawMessage, error)
}

// esClient implements the Client interface
type esClient struct {
	client *elasticsearch.Client
	index  string
}

// NewClient creates a new Elasticsearch client
func NewClient(cfg config.ElasticsearchConfig) (Client, error) {
	// Create Elasticsearch config
	esCfg := elasticsearch.Config{
		Addresses: cfg.URLs,
	}
	
	// Add authentication if provided
	if cfg.Username != "" && cfg.Password != "" {
		esCfg.Username = cfg.Username
		esCfg.Password = cfg.Password
	}
	
	// Configure TLS for secure connections
	esCfg.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
	
	// Create the client
	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}
	
	// Test the connection
	res, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Elasticsearch: %w", err)
	}
	defer res.Body.Close()
	
	if res.IsError() {
		return nil, fmt.Errorf("Elasticsearch error: %s", res.String())
	}
	
	return &esClient{
		client: client,
		index:  cfg.Index,
	}, nil
}

// IndexDocument indexes a document in Elasticsearch
func (e *esClient) IndexDocument(ctx context.Context, id string, document []byte) error {
	// Create request with context
	req := esapi.IndexRequest{
		Index:      e.index,
		DocumentID: id,
		Body:       bytes.NewReader(document),
		Refresh:    "true",
	}
	
	// Execute the request
	res, err := req.Do(ctx, e.client)
	if err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}
	defer res.Body.Close()
	
	// Check for errors
	if res.IsError() {
		return fmt.Errorf("error indexing document: %s", res.String())
	}
	
	return nil
}

// SearchDocuments searches for documents in Elasticsearch
func (e *esClient) SearchDocuments(ctx context.Context, query interface{}) ([]json.RawMessage, error) {
	// Convert query to JSON
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, fmt.Errorf("failed to encode query: %w", err)
	}
	
	// Perform the search request
	res, err := e.client.Search(
		e.client.Search.WithContext(ctx),
		e.client.Search.WithIndex(e.index),
		e.client.Search.WithBody(&buf),
		e.client.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}
	defer res.Body.Close()
	
	// Check for errors
	if res.IsError() {
		return nil, fmt.Errorf("error searching documents: %s", res.String())
	}
	
	// Parse the response
	var result struct {
		Hits struct {
			Hits []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}
	
	// Extract the documents
	docs := make([]json.RawMessage, len(result.Hits.Hits))
	for i, hit := range result.Hits.Hits {
		docs[i] = hit.Source
	}
	
	return docs, nil
}

// GetDocument retrieves a document from Elasticsearch by ID
func (e *esClient) GetDocument(ctx context.Context, id string) (json.RawMessage, error) {
	// Perform the get request
	res, err := e.client.Get(
		e.index,
		id,
		e.client.Get.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}
	defer res.Body.Close()
	
	// Check if document exists
	if res.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("document not found")
	}
	
	// Check for other errors
	if res.IsError() {
		return nil, fmt.Errorf("error getting document: %s", res.String())
	}
	
	// Parse the response
	var result struct {
		Found  bool            `json:"found"`
		Source json.RawMessage `json:"_source"`
	}
	
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse get response: %w", err)
	}
	
	if !result.Found {
		return nil, fmt.Errorf("document not found")
	}
	
	return result.Source, nil
}
