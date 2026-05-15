package config

import "encoding/json"

// SyncMode defines the synchronisation direction for a watched folder.
type SyncMode int

const (
	// ModeTwoWay synchronises both local and remote changes.
	ModeTwoWay SyncMode = iota
	// ModeUploadOnly uploads local changes but never downloads remote changes.
	ModeUploadOnly
	// ModeDownloadOnly downloads remote changes but never uploads local changes.
	ModeDownloadOnly
)

// String returns the JSON-friendly representation.
func (m SyncMode) String() string {
	switch m {
	case ModeUploadOnly:
		return "upload_only"
	case ModeDownloadOnly:
		return "download_only"
	default:
		return "two_way"
	}
}

// MarshalJSON serialises the mode as a JSON string.
func (m SyncMode) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}

// UnmarshalJSON deserialises a JSON string into a SyncMode.
func (m *SyncMode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*m = ParseSyncMode(s)
	return nil
}

// ParseSyncMode converts a string to SyncMode, defaulting to ModeTwoWay for
// unrecognised values. This guarantees backward compatibility with older configs.
func ParseSyncMode(s string) SyncMode {
	switch s {
	case "upload_only":
		return ModeUploadOnly
	case "download_only":
		return ModeDownloadOnly
	default:
		return ModeTwoWay
	}
}

// ---------------------------------------------------------------------------
// Mode capability helpers – deterministic, side-effect free.
// ---------------------------------------------------------------------------

// AllowsUpload returns true if the mode permits uploading local files to Drive.
func (m SyncMode) AllowsUpload() bool {
	return m == ModeTwoWay || m == ModeUploadOnly
}

// AllowsDownload returns true if the mode permits downloading remote files locally.
func (m SyncMode) AllowsDownload() bool {
	return m == ModeTwoWay || m == ModeDownloadOnly
}

// AllowsConflictResolution returns true if the mode should run the conflict resolver.
// Conflicts only make sense when both sides are being synchronised.
func (m SyncMode) AllowsConflictResolution() bool {
	return m == ModeTwoWay
}

// ShouldProcessLocalEvents returns true if local filesystem events should be acted upon.
func (m SyncMode) ShouldProcessLocalEvents() bool {
	return m == ModeTwoWay || m == ModeUploadOnly
}

// ShouldProcessRemoteEvents returns true if remote Drive changes should be applied locally.
func (m SyncMode) ShouldProcessRemoteEvents() bool {
	return m == ModeTwoWay || m == ModeDownloadOnly
}
