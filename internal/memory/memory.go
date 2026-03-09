// Package memory implements the shared memory layer for managing context and artifact sharing between agents.
package memory

import "github.com/clawfactory/clawfactory/internal/model"

// SharedMemory is the shared memory layer interface.
type SharedMemory interface {
	StoreArtifact(workflowID, taskID, name string, data []byte) (model.Artifact, error)
	GetArtifacts(workflowID string) ([]model.Artifact, error)
	GetArtifactsByTask(workflowID, taskID string) ([]model.Artifact, error)
	GetUpstreamArtifacts(workflowID, taskID string) ([]model.Artifact, error)
	ReadArtifact(artifact model.Artifact) ([]byte, error)
}
