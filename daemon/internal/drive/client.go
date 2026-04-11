package drive

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	gdrive "google.golang.org/api/drive/v3"

	"github.com/synca/daemon/internal/auth"
)

// File represents a Drive file with metadata we care about.
type File struct {
	ID       string
	Name     string
	MimeType string
	ModTime  time.Time
	Size     int64
	MD5      string
	Parents  []string
}

// Client wraps the Google Drive API.
type Client struct {
	svc *gdrive.Service
}

// ✅ Agora usa o auth centralizado (SEM duplicação, SEM bug)
func NewClient(ctx context.Context) (*Client, error) {
	svc, err := auth.NewDriveService(ctx)
	if err != nil {
		return nil, err
	}
	return &Client{svc: svc}, nil
}

// ListFiles returns all files in the given Drive folder (root if empty).
func (c *Client) ListFiles(ctx context.Context, folderID string) ([]*File, error) {
	if folderID == "" {
		folderID = "root"
	}

	query := fmt.Sprintf("'%s' in parents and trashed=false", folderID)

	var files []*File
	pageToken := ""

	for {
		call := c.svc.Files.List().
		Q(query).
		Fields("nextPageToken, files(id,name,mimeType,modifiedTime,size,md5Checksum,parents)").
		Context(ctx)

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		result, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("drive.ListFiles: %w", err)
		}

		for _, f := range result.Files {
			files = append(files, driveFileToFile(f))
		}

		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}

	return files, nil
}

// UploadFile uploads or updates a file in Drive.
func (c *Client) UploadFile(ctx context.Context, localPath, remoteName, parentID, remoteID string) (*File, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, _ := f.Stat()

	log.Info().
	Str("file", remoteName).
	Int64("size", info.Size()).
	Msg("Uploading to Drive")

	meta := &gdrive.File{Name: remoteName}
	if parentID != "" {
		meta.Parents = []string{parentID}
	}

	var result *gdrive.File

	if remoteID == "" {
		result, err = c.svc.Files.Create(meta).
		Media(f).
		Fields("id,name,mimeType,modifiedTime,size,md5Checksum,parents").
		Context(ctx).
		Do()
	} else {
		result, err = c.svc.Files.Update(remoteID, &gdrive.File{}).
		Media(f).
		Fields("id,name,mimeType,modifiedTime,size,md5Checksum,parents").
		Context(ctx).
		Do()
	}

	if err != nil {
		return nil, fmt.Errorf("drive.Upload(%s): %w", remoteName, err)
	}

	return driveFileToFile(result), nil
}

// DownloadFile downloads a Drive file to local disk.
func (c *Client) DownloadFile(ctx context.Context, fileID, destPath string) error {
	resp, err := c.svc.Files.Get(fileID).Context(ctx).Download()
	if err != nil {
		return fmt.Errorf("drive.Download(%s): %w", fileID, err)
	}
	defer resp.Body.Close()

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	dest, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, resp.Body)
	return err
}

// DeleteFile moves a file to trash.
func (c *Client) DeleteFile(ctx context.Context, fileID string) error {
	_, err := c.svc.Files.Update(fileID, &gdrive.File{Trashed: true}).Context(ctx).Do()
	return err
}

// GetOrCreateFolder finds or creates a folder in Drive.
func (c *Client) GetOrCreateFolder(ctx context.Context, name, parentID string) (string, error) {
	if parentID == "" {
		parentID = "root"
	}

	query := fmt.Sprintf(
		"name='%s' and '%s' in parents and mimeType='application/vnd.google-apps.folder' and trashed=false",
		name, parentID,
	)

	result, err := c.svc.Files.List().
	Q(query).
	Fields("files(id)").
	Context(ctx).
	Do()

	if err != nil {
		return "", err
	}

	if len(result.Files) > 0 {
		return result.Files[0].Id, nil
	}

	folder := &gdrive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  []string{parentID},
	}

	created, err := c.svc.Files.Create(folder).
	Fields("id").
	Context(ctx).
	Do()

	if err != nil {
		return "", err
	}

	return created.Id, nil
}

// GetFileByNameInFolder finds an existing file in the given parent folder to avoid duplicates.
func (c *Client) GetFileByNameInFolder(ctx context.Context, name, parentID string) (*File, error) {
	if parentID == "" {
		parentID = "root"
	}
	query := fmt.Sprintf("name='%s' and '%s' in parents and mimeType!='application/vnd.google-apps.folder' and trashed=false", name, parentID)

	result, err := c.svc.Files.List().
		Q(query).
		Fields("files(id,name,mimeType,modifiedTime,size,md5Checksum,parents)").
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}
	if len(result.Files) > 0 {
		return driveFileToFile(result.Files[0]), nil
	}
	return nil, nil
}

// internal mapper
func driveFileToFile(f *gdrive.File) *File {
	t, _ := time.Parse(time.RFC3339, f.ModifiedTime)

	return &File{
		ID:       f.Id,
		Name:     f.Name,
		MimeType: f.MimeType,
		ModTime:  t,
		Size:     f.Size,
		MD5:      f.Md5Checksum,
		Parents:  f.Parents,
	}
}
