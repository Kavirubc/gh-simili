// Author: Kaviru Hapuarachchi
// GitHub: https://github.com/Kavirubc
// Created: 2026-01-28
// Last Modified: 2026-01-28

package pipeline

import (
	"fmt"

	"github.com/Kavirubc/gh-simili/internal/config"
	"github.com/Kavirubc/gh-simili/internal/github"
	"github.com/Kavirubc/gh-simili/internal/pipeline/core"
	"github.com/Kavirubc/gh-simili/internal/pipeline/steps"
	"github.com/Kavirubc/gh-simili/internal/processor"
	"github.com/Kavirubc/gh-simili/internal/triage"
	"github.com/Kavirubc/gh-simili/internal/vectordb"
)

// Builder constructs a pipeline of steps.
type Builder struct {
	cfg            *config.Config
	gh             *github.Client
	transferClient *github.Client
	vdb            *vectordb.Client
	similarity     *processor.SimilarityFinder
	indexer        *processor.Indexer
	triageAgent    *triage.Agent
	dryRun         bool
	execute        bool
}

// NewBuilder creates a new pipeline builder
func NewBuilder(
	cfg *config.Config,
	gh *github.Client,
	transferClient *github.Client,
	vdb *vectordb.Client,
	similarity *processor.SimilarityFinder,
	indexer *processor.Indexer,
	triageAgent *triage.Agent,
	dryRun bool,
	execute bool,
) *Builder {
	return &Builder{
		cfg:            cfg,
		gh:             gh,
		transferClient: transferClient,
		vdb:            vdb,
		similarity:     similarity,
		indexer:        indexer,
		triageAgent:    triageAgent,
		dryRun:         dryRun,
		execute:        execute,
	}
}

// BuildDefault creates the standard pipeline
func (b *Builder) BuildDefault() []core.Step {
	return []core.Step{
		steps.NewRepoGatekeeper(b.gh),
		steps.NewVectorDBPrep(b.vdb, b.dryRun),
		steps.NewSimilaritySearch(b.similarity),
		steps.NewTransferCheck(),
		steps.NewTriageAnalysis(b.triageAgent),
		steps.NewResponseBuilder(),
		steps.NewActionExecutor(b.gh, b.transferClient, b.vdb, b.dryRun, b.execute),
		steps.NewIndexer(b.indexer, b.dryRun),
	}
}

// BuildFromConfig creates a pipeline based on the order defined in config.
// If config is empty, returns default.
func (b *Builder) BuildFromConfig() ([]core.Step, error) {
	if len(b.cfg.Pipeline.Steps) == 0 {
		return b.BuildDefault(), nil
	}

	var pipe []core.Step
	for _, name := range b.cfg.Pipeline.Steps {
		step, err := b.createStep(name)
		if err != nil {
			return nil, err
		}
		pipe = append(pipe, step)
	}
	return pipe, nil
}

func (b *Builder) createStep(name string) (core.Step, error) {
	switch name {
	case "gatekeeper":
		return steps.NewRepoGatekeeper(b.gh), nil
	case "vectordb_prep":
		return steps.NewVectorDBPrep(b.vdb, b.dryRun), nil
	case "similarity_search":
		return steps.NewSimilaritySearch(b.similarity), nil
	case "transfer_check":
		return steps.NewTransferCheck(), nil
	case "triage":
		return steps.NewTriageAnalysis(b.triageAgent), nil
	case "response_builder":
		return steps.NewResponseBuilder(), nil
	case "action_executor":
		return steps.NewActionExecutor(b.gh, b.transferClient, b.vdb, b.dryRun, b.execute), nil
	case "indexer":
		return steps.NewIndexer(b.indexer, b.dryRun), nil
	default:
		return nil, fmt.Errorf("unknown step: %s", name)
	}
}
