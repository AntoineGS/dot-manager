package forms

import "github.com/AntoineGS/tidydots/internal/config"

// ToggleCheckMode switches between the two setup check-mode interpretations.
// An untouched omission remains empty until the user interacts with the field.
func (f *SubEntryForm) ToggleCheckMode() {
	if f == nil || !f.IsSetup {
		return
	}
	if f.CheckMode == config.CheckModeStatus {
		f.CheckMode = config.CheckModeExitCode
	} else {
		f.CheckMode = config.CheckModeStatus
	}
}
