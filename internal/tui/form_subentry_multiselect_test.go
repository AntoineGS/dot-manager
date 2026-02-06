package tui

import (
	"path/filepath"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	testNvimDir = "/home/user/.config/nvim"
	testInitLua = "init.lua"
)

// TestMultiSelect_ToggleSelection tests toggling a file selection with space/tab
func TestMultiSelect_ToggleSelection(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name:        "test-app",
				Description: "Test application",
				Entries: []config.SubEntry{
					{
						Name:    "placeholder",
						Targets: map[string]string{"linux": "/tmp/test"},
					},
				},
			},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)
	m.initSubEntryFormNew(0)

	// Set up file picker mode with a simulated current directory and path
	m.subEntryForm.addFileMode = ModePicker
	m.subEntryForm.filePicker.CurrentDirectory = testNvimDir
	m.subEntryForm.filePicker.Path = testInitLua

	initialCount := len(m.subEntryForm.selectedFiles)

	// Simulate space key to toggle selection
	updatedModel, _ := m.updateSubEntryFilePicker(createKeyMsg(" "))
	model := updatedModel.(Model)

	// Verify selection was added
	expectedPath := filepath.Join(testNvimDir, testInitLua)
	if !model.subEntryForm.selectedFiles[expectedPath] {
		t.Errorf("file not selected: %s", expectedPath)
	}

	if len(model.subEntryForm.selectedFiles) != initialCount+1 {
		t.Errorf("selectedFiles count = %d, want %d", len(model.subEntryForm.selectedFiles), initialCount+1)
	}

	// Toggle again to deselect
	updatedModel2, _ := model.updateSubEntryFilePicker(createKeyMsg(" "))
	model2 := updatedModel2.(Model)

	if model2.subEntryForm.selectedFiles[expectedPath] {
		t.Errorf("file still selected after toggle: %s", expectedPath)
	}

	if len(model2.subEntryForm.selectedFiles) != initialCount {
		t.Errorf("selectedFiles count = %d, want %d after deselect", len(model2.subEntryForm.selectedFiles), initialCount)
	}
}

// TestMultiSelect_ToggleWithTab tests toggling selection with tab key
func TestMultiSelect_ToggleWithTab(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name:        "test-app",
				Description: "Test application",
				Entries: []config.SubEntry{
					{
						Name:    "placeholder",
						Targets: map[string]string{"linux": "/tmp/test"},
					},
				},
			},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)
	m.initSubEntryFormNew(0)

	// Set up file picker mode
	m.subEntryForm.addFileMode = ModePicker
	m.subEntryForm.filePicker.CurrentDirectory = testNvimDir
	m.subEntryForm.filePicker.Path = testInitLua

	// Tab key should also toggle selection
	updatedModel, _ := m.updateSubEntryFilePicker(tea.KeyMsg{Type: tea.KeyTab})
	model := updatedModel.(Model)

	expectedPath := filepath.Join(testNvimDir, testInitLua)
	if !model.subEntryForm.selectedFiles[expectedPath] {
		t.Errorf("file not selected with tab key: %s", expectedPath)
	}
}

// TestMultiSelect_PersistAcrossNavigation tests selections persist when navigating
func TestMultiSelect_PersistAcrossNavigation(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name:        "test-app",
				Description: "Test application",
				Entries: []config.SubEntry{
					{
						Name:    "placeholder",
						Targets: map[string]string{"linux": "/tmp/test"},
					},
				},
			},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)
	m.initSubEntryFormNew(0)

	// Select first file
	m.subEntryForm.addFileMode = ModePicker
	m.subEntryForm.filePicker.CurrentDirectory = testNvimDir
	m.subEntryForm.filePicker.Path = testInitLua

	updatedModel, _ := m.updateSubEntryFilePicker(createKeyMsg(" "))
	model := updatedModel.(Model)

	file1Path := filepath.Join(testNvimDir, testInitLua)

	// Simulate navigation to another file
	model.subEntryForm.filePicker.Path = "plugins.lua"

	// Select second file
	updatedModel2, _ := model.updateSubEntryFilePicker(createKeyMsg(" "))
	model2 := updatedModel2.(Model)

	file2Path := filepath.Join(testNvimDir, "plugins.lua")

	// Verify both files are still selected
	if !model2.subEntryForm.selectedFiles[file1Path] {
		t.Errorf("first file not selected after navigation: %s", file1Path)
	}

	if !model2.subEntryForm.selectedFiles[file2Path] {
		t.Errorf("second file not selected: %s", file2Path)
	}

	if len(model2.subEntryForm.selectedFiles) != 2 {
		t.Errorf("selectedFiles count = %d, want 2", len(model2.subEntryForm.selectedFiles))
	}
}

// TestMultiSelect_ConfirmMultipleFiles tests confirming multiple selections with enter
func TestMultiSelect_ConfirmMultipleFiles(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name:        "test-app",
				Description: "Test application",
				Entries: []config.SubEntry{
					{
						Name:    "placeholder",
						Targets: map[string]string{"linux": "/tmp/test"},
					},
				},
			},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)
	m.initSubEntryFormNew(0)

	// Set up target path
	m.subEntryForm.linuxTargetInput.SetValue(testNvimDir)

	// Pre-populate selectedFiles map with multiple selections
	m.subEntryForm.addFileMode = ModePicker
	m.subEntryForm.selectedFiles[testNvimDir+"/"+testInitLua] = true
	m.subEntryForm.selectedFiles[testNvimDir+"/plugins.lua"] = true
	m.subEntryForm.selectedFiles[testNvimDir+"/lua/config.lua"] = true

	initialFilesCount := len(m.subEntryForm.files)

	// Simulate enter key to confirm selections
	updatedModel, _ := m.updateSubEntryFilePicker(createKeyMsg(KeyEnter))
	model := updatedModel.(Model)

	// Verify all files were added to the files list
	expectedCount := initialFilesCount + 3
	if len(model.subEntryForm.files) != expectedCount {
		t.Errorf("files count = %d, want %d", len(model.subEntryForm.files), expectedCount)
	}

	// Verify selectedFiles map was cleared
	if len(model.subEntryForm.selectedFiles) != 0 {
		t.Errorf("selectedFiles not cleared: count = %d, want 0", len(model.subEntryForm.selectedFiles))
	}

	// Verify mode was reset
	if model.subEntryForm.addFileMode != ModeNone {
		t.Errorf("addFileMode = %d, want %d (ModeNone)", model.subEntryForm.addFileMode, ModeNone)
	}

	// Verify files contain expected relative paths
	expectedFiles := []string{testInitLua, "plugins.lua", "lua/config.lua"}
	for _, expected := range expectedFiles {
		found := false
		for _, file := range model.subEntryForm.files {
			if file == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected file not found in files list: %s", expected)
		}
	}
}

// TestMultiSelect_EmptyConfirm tests confirming with no selections
func TestMultiSelect_EmptyConfirm(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name:        "test-app",
				Description: "Test application",
				Entries: []config.SubEntry{
					{
						Name:    "placeholder",
						Targets: map[string]string{"linux": "/tmp/test"},
					},
				},
			},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)
	m.initSubEntryFormNew(0)

	// Set up file picker mode with no selections
	m.subEntryForm.addFileMode = ModePicker
	m.subEntryForm.filePicker.CurrentDirectory = testNvimDir
	m.subEntryForm.filePicker.Path = ""

	initialFilesCount := len(m.subEntryForm.files)

	// Simulate enter key with no selections
	updatedModel, _ := m.updateSubEntryFilePicker(createKeyMsg(KeyEnter))
	model := updatedModel.(Model)

	// Verify no files were added
	if len(model.subEntryForm.files) != initialFilesCount {
		t.Errorf("files count changed: got %d, want %d", len(model.subEntryForm.files), initialFilesCount)
	}

	// Verify mode was reset
	if model.subEntryForm.addFileMode != ModeNone {
		t.Errorf("addFileMode = %d, want %d (ModeNone)", model.subEntryForm.addFileMode, ModeNone)
	}
}

// TestMultiSelect_CancelPreservesNoFiles tests that canceling doesn't add files
func TestMultiSelect_CancelPreservesNoFiles(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name:        "test-app",
				Description: "Test application",
				Entries: []config.SubEntry{
					{
						Name:    "placeholder",
						Targets: map[string]string{"linux": "/tmp/test"},
					},
				},
			},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)
	m.initSubEntryFormNew(0)

	// Set up file picker mode with selections
	m.subEntryForm.addFileMode = ModePicker
	m.subEntryForm.selectedFiles[testNvimDir+"/"+testInitLua] = true
	m.subEntryForm.selectedFiles[testNvimDir+"/plugins.lua"] = true

	initialFilesCount := len(m.subEntryForm.files)

	// Simulate ESC key to cancel
	updatedModel, _ := m.updateSubEntryFilePicker(createKeyMsg(KeyEsc))
	model := updatedModel.(Model)

	// Verify no files were added
	if len(model.subEntryForm.files) != initialFilesCount {
		t.Errorf("files count changed after cancel: got %d, want %d", len(model.subEntryForm.files), initialFilesCount)
	}

	// Verify selections were cleared
	if len(model.subEntryForm.selectedFiles) != 0 {
		t.Errorf("selectedFiles not cleared after cancel: count = %d, want 0", len(model.subEntryForm.selectedFiles))
	}

	// Verify mode was reset
	if model.subEntryForm.addFileMode != ModeNone {
		t.Errorf("addFileMode = %d, want %d (ModeNone)", model.subEntryForm.addFileMode, ModeNone)
	}
}

// TestMultiSelect_SelectionCount tests tracking the number of selections
func TestMultiSelect_SelectionCount(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name:        "test-app",
				Description: "Test application",
				Entries: []config.SubEntry{
					{
						Name:    "placeholder",
						Targets: map[string]string{"linux": "/tmp/test"},
					},
				},
			},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)
	m.initSubEntryFormNew(0)

	// Start with empty selections
	if len(m.subEntryForm.selectedFiles) != 0 {
		t.Errorf("initial selectedFiles count = %d, want 0", len(m.subEntryForm.selectedFiles))
	}

	// Add selections
	m.subEntryForm.selectedFiles[testNvimDir+"/"+testInitLua] = true
	m.subEntryForm.selectedFiles[testNvimDir+"/plugins.lua"] = true
	m.subEntryForm.selectedFiles[testNvimDir+"/lua/config.lua"] = true

	// Verify count
	if len(m.subEntryForm.selectedFiles) != 3 {
		t.Errorf("selectedFiles count = %d, want 3", len(m.subEntryForm.selectedFiles))
	}

	// Remove one selection
	delete(m.subEntryForm.selectedFiles, testNvimDir+"/plugins.lua")

	// Verify count updated
	if len(m.subEntryForm.selectedFiles) != 2 {
		t.Errorf("selectedFiles count after delete = %d, want 2", len(m.subEntryForm.selectedFiles))
	}
}

// TestMultiSelect_DuplicateSelectionIgnored tests that selecting same file twice doesn't duplicate
func TestMultiSelect_DuplicateSelectionIgnored(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name:        "test-app",
				Description: "Test application",
				Entries: []config.SubEntry{
					{
						Name:    "placeholder",
						Targets: map[string]string{"linux": "/tmp/test"},
					},
				},
			},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)
	m.initSubEntryFormNew(0)

	filePath := testNvimDir + "/" + testInitLua

	// Select file twice
	m.subEntryForm.selectedFiles[filePath] = true
	m.subEntryForm.selectedFiles[filePath] = true

	// Verify only one selection
	if len(m.subEntryForm.selectedFiles) != 1 {
		t.Errorf("selectedFiles count = %d, want 1", len(m.subEntryForm.selectedFiles))
	}

	if !m.subEntryForm.selectedFiles[filePath] {
		t.Errorf("file not selected: %s", filePath)
	}
}

// TestMultiSelect_MixedSelectionsAndConfirm tests selecting, deselecting, and confirming
func TestMultiSelect_MixedSelectionsAndConfirm(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name:        "test-app",
				Description: "Test application",
				Entries: []config.SubEntry{
					{
						Name:    "placeholder",
						Targets: map[string]string{"linux": "/tmp/test"},
					},
				},
			},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)
	m.initSubEntryFormNew(0)

	// Set up target path
	m.subEntryForm.linuxTargetInput.SetValue(testNvimDir)

	// Select multiple files
	m.subEntryForm.selectedFiles[testNvimDir+"/"+testInitLua] = true
	m.subEntryForm.selectedFiles[testNvimDir+"/plugins.lua"] = true
	m.subEntryForm.selectedFiles[testNvimDir+"/colors.lua"] = true

	// Deselect one
	delete(m.subEntryForm.selectedFiles, testNvimDir+"/colors.lua")

	// Verify count before confirm
	if len(m.subEntryForm.selectedFiles) != 2 {
		t.Errorf("selectedFiles count before confirm = %d, want 2", len(m.subEntryForm.selectedFiles))
	}

	// Confirm selections
	m.subEntryForm.addFileMode = ModePicker
	updatedModel, _ := m.updateSubEntryFilePicker(createKeyMsg(KeyEnter))
	model := updatedModel.(Model)

	// Verify only 2 files were added (not 3)
	if len(model.subEntryForm.files) != 2 {
		t.Errorf("files count = %d, want 2", len(model.subEntryForm.files))
	}

	// Verify correct files were added
	expectedFiles := []string{testInitLua, "plugins.lua"}
	for _, expected := range expectedFiles {
		found := false
		for _, file := range model.subEntryForm.files {
			if file == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected file not found in files list: %s", expected)
		}
	}

	// Verify deselected file was not added
	for _, file := range model.subEntryForm.files {
		if file == "colors.lua" {
			t.Errorf("deselected file was added: colors.lua")
		}
	}
}
