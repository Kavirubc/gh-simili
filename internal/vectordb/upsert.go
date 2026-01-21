package vectordb

import (
	"context"
	"fmt"
	"time"

	"github.com/kaviruhapuarachchi/gh-simili/pkg/models"
	"github.com/qdrant/go-client/qdrant"
)

// Upsert inserts or updates a single issue vector
func (c *Client) Upsert(ctx context.Context, collection string, issue *models.Issue, vector []float32) error {
	point := issueToPoint(issue, vector)

	_, err := c.qdrant.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Points:         []*qdrant.PointStruct{point},
	})
	if err != nil {
		return fmt.Errorf("upsert failed: %w", err)
	}
	return nil
}

// UpsertBatch inserts or updates multiple issue vectors
func (c *Client) UpsertBatch(ctx context.Context, collection string, issues []*models.Issue, vectors [][]float32) error {
	if len(issues) != len(vectors) {
		return fmt.Errorf("issues and vectors length mismatch")
	}

	points := make([]*qdrant.PointStruct, len(issues))
	for i, issue := range issues {
		points[i] = issueToPoint(issue, vectors[i])
	}

	_, err := c.qdrant.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("batch upsert failed: %w", err)
	}
	return nil
}

// Delete removes a point by ID
func (c *Client) Delete(ctx context.Context, collection string, id string) error {
	_, err := c.qdrant.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{
					Ids: []*qdrant.PointId{qdrant.NewIDUUID(id)},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	return nil
}

// DeleteBatch removes multiple points by ID
func (c *Client) DeleteBatch(ctx context.Context, collection string, ids []string) error {
	pointIds := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		pointIds[i] = qdrant.NewIDUUID(id)
	}

	_, err := c.qdrant.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{
					Ids: pointIds,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("batch delete failed: %w", err)
	}
	return nil
}

// issueToPoint converts an Issue to a Qdrant point
func issueToPoint(issue *models.Issue, vector []float32) *qdrant.PointStruct {
	labelValues := make([]*qdrant.Value, len(issue.Labels))
	for i, label := range issue.Labels {
		labelValues[i] = qdrant.NewValueString(label)
	}

	return &qdrant.PointStruct{
		Id:      qdrant.NewIDUUID(issue.UUID()),
		Vectors: qdrant.NewVectors(vector...),
		Payload: map[string]*qdrant.Value{
			"org":        qdrant.NewValueString(issue.Org),
			"repo":       qdrant.NewValueString(issue.Repo),
			"number":     qdrant.NewValueInt(int64(issue.Number)),
			"title":      qdrant.NewValueString(issue.Title),
			"state":      qdrant.NewValueString(issue.State),
			"author":     qdrant.NewValueString(issue.Author),
			"url":        qdrant.NewValueString(issue.URL),
			"body_hash":  qdrant.NewValueString(issue.BodyHash()),
			"created_at": qdrant.NewValueString(issue.CreatedAt.Format(time.RFC3339)),
			"updated_at": qdrant.NewValueString(issue.UpdatedAt.Format(time.RFC3339)),
			"labels": &qdrant.Value{
				Kind: &qdrant.Value_ListValue{
					ListValue: &qdrant.ListValue{Values: labelValues},
				},
			},
		},
	}
}
