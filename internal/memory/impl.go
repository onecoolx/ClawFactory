package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
)

// FileSystemMemory 基于文件系统的共享记忆层实现
type FileSystemMemory struct {
	dataDir string
	store   store.StateStore
}

// NewFileSystemMemory 创建文件系统共享记忆层
func NewFileSystemMemory(dataDir string, s store.StateStore) *FileSystemMemory {
	return &FileSystemMemory{dataDir: dataDir, store: s}
}

func (m *FileSystemMemory) artifactDir(workflowID, taskID string) string {
	return filepath.Join(m.dataDir, "artifacts", workflowID, taskID)
}

func (m *FileSystemMemory) StoreArtifact(workflowID, taskID, name string, data []byte) (model.Artifact, error) {
	dir := m.artifactDir(workflowID, taskID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return model.Artifact{}, fmt.Errorf("mkdir: %w", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return model.Artifact{}, fmt.Errorf("write: %w", err)
	}
	artifact := model.Artifact{
		WorkflowID: workflowID, TaskID: taskID, Name: name,
		Path: path, CreatedAt: time.Now(),
	}
	if err := m.store.SaveArtifact(artifact); err != nil {
		return model.Artifact{}, fmt.Errorf("save metadata: %w", err)
	}
	return artifact, nil
}

func (m *FileSystemMemory) GetArtifacts(workflowID string) ([]model.Artifact, error) {
	return m.store.GetArtifacts(workflowID)
}

func (m *FileSystemMemory) GetArtifactsByTask(workflowID, taskID string) ([]model.Artifact, error) {
	all, err := m.store.GetArtifacts(workflowID)
	if err != nil {
		return nil, err
	}
	var result []model.Artifact
	for _, a := range all {
		if a.TaskID == taskID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *FileSystemMemory) GetUpstreamArtifacts(workflowID, taskID string) ([]model.Artifact, error) {
	all, err := m.store.GetArtifacts(workflowID)
	if err != nil {
		return nil, err
	}
	var result []model.Artifact
	for _, a := range all {
		if a.TaskID != taskID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *FileSystemMemory) ReadArtifact(artifact model.Artifact) ([]byte, error) {
	return os.ReadFile(artifact.Path)
}
