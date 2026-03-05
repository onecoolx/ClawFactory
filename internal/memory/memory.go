// Package memory 实现共享记忆层，管理智能体之间的上下文和产出物共享
package memory

import "github.com/clawfactory/clawfactory/internal/model"

// SharedMemory 共享记忆层接口
type SharedMemory interface {
	StoreArtifact(workflowID, taskID, name string, data []byte) (model.Artifact, error)
	GetArtifacts(workflowID string) ([]model.Artifact, error)
	GetArtifactsByTask(workflowID, taskID string) ([]model.Artifact, error)
	GetUpstreamArtifacts(workflowID, taskID string) ([]model.Artifact, error)
	ReadArtifact(artifact model.Artifact) ([]byte, error)
}
