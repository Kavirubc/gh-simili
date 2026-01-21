package vectordb

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
)

const vectorDimensions = 768

// EnsureCollection creates collection if it doesn't exist
func (c *Client) EnsureCollection(ctx context.Context, name string) error {
	// Check if collection exists
	exists, err := c.qdrant.CollectionExists(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to check collection: %w", err)
	}

	if exists {
		return nil
	}

	// Create collection
	err = c.qdrant.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     vectorDimensions,
			Distance: qdrant.Distance_Cosine,
		}),
	})
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	// Create payload indexes for filtering
	indexes := []struct {
		field     string
		fieldType qdrant.FieldType
	}{
		{"org", qdrant.FieldType_FieldTypeKeyword},
		{"repo", qdrant.FieldType_FieldTypeKeyword},
		{"state", qdrant.FieldType_FieldTypeKeyword},
		{"number", qdrant.FieldType_FieldTypeInteger},
		{"labels", qdrant.FieldType_FieldTypeKeyword},
	}

	for _, idx := range indexes {
		_, err = c.qdrant.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
			CollectionName: name,
			FieldName:      idx.field,
			FieldType:      qdrant.PtrOf(idx.fieldType),
		})
		if err != nil {
			// Index creation failure is not fatal
			fmt.Printf("Warning: failed to create index for %s: %v\n", idx.field, err)
		}
	}

	return nil
}

// DeleteCollection removes a collection
func (c *Client) DeleteCollection(ctx context.Context, name string) error {
	return c.qdrant.DeleteCollection(ctx, name)
}

// CollectionExists checks if a collection exists
func (c *Client) CollectionExists(ctx context.Context, name string) (bool, error) {
	return c.qdrant.CollectionExists(ctx, name)
}
