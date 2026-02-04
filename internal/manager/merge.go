package manager

// MergeSummary tracks merge operations for a single application.
// This type is not thread-safe and should not be used concurrently.
type MergeSummary struct {
	AppName       string
	MergedFiles   []string
	ConflictFiles []ConflictInfo
	FailedFiles   []FailedInfo
}

// ConflictInfo tracks files that were renamed due to conflicts
type ConflictInfo struct {
	OriginalName string
	RenamedTo    string
}

// FailedInfo tracks files that failed to merge
type FailedInfo struct {
	FileName string
	Error    string
}

// NewMergeSummary creates a new merge summary for an application
func NewMergeSummary(appName string) *MergeSummary {
	return &MergeSummary{
		AppName:       appName,
		MergedFiles:   []string{},
		ConflictFiles: []ConflictInfo{},
		FailedFiles:   []FailedInfo{},
	}
}

// AddMerged records a successfully merged file
func (s *MergeSummary) AddMerged(fileName string) {
	s.MergedFiles = append(s.MergedFiles, fileName)
}

// AddConflict records a conflict that was resolved by renaming
func (s *MergeSummary) AddConflict(originalName, renamedTo string) {
	s.ConflictFiles = append(s.ConflictFiles, ConflictInfo{
		OriginalName: originalName,
		RenamedTo:    renamedTo,
	})
}

// AddFailed records a file that failed to merge
func (s *MergeSummary) AddFailed(fileName, errMsg string) {
	s.FailedFiles = append(s.FailedFiles, FailedInfo{
		FileName: fileName,
		Error:    errMsg,
	})
}

// HasOperations returns true if any merge operations occurred
func (s *MergeSummary) HasOperations() bool {
	return len(s.MergedFiles) > 0 || len(s.ConflictFiles) > 0 || len(s.FailedFiles) > 0
}
