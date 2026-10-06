package workspace

import (
	"errors"
	"fmt"
	"os"
)

// FileResult reports what saving one dirty source did. Backup is the
// timestamped backup the save created ("", when the file did not exist
// before). Stale marks a stale-guard refusal: the file changed on disk
// since load, its bytes are untouched, and its dirty set is preserved. Err
// carries any other save failure; its dirty set is preserved too.
type FileResult struct {
	SourceID string
	Path     string
	Backup   string
	Stale    bool
	Err      error
}

// SaveResult lists one FileResult per dirty source SaveAll attempted, in
// registry order. Clean sources are never written and never listed.
type SaveResult struct {
	Files []FileResult
}

// OK reports whether every attempted file saved cleanly.
func (r SaveResult) OK() bool {
	for _, file := range r.Files {
		if file.Err != nil {
			return false
		}
	}
	return true
}

// fileSnapshot pins what a source's file looked like when the registry last
// saw a consistent state: at load, at attach, after a successful save, or
// after a reload. The guard compares modification time and size.
type fileSnapshot struct {
	mtime  int64
	size   int64
	exists bool
}

func snapshotOf(path string) fileSnapshot {
	info, err := os.Stat(path)
	if err != nil {
		return fileSnapshot{}
	}
	return fileSnapshot{mtime: info.ModTime().UnixNano(), size: info.Size(), exists: true}
}

// remember records the consistent-state snapshot of a source's file. A file
// that does not exist has no state to guard, so its snapshot is cleared
// instead: mcp.json before its first add can then be created by the first
// save rather than being refused as stale. Once the file exists, every save
// and reload refreshes a real snapshot.
func (r *Registry) remember(id, path string) {
	snapshot := snapshotOf(path)
	if !snapshot.exists {
		delete(r.snapshots, id)
		return
	}
	if r.snapshots == nil {
		r.snapshots = make(map[string]fileSnapshot)
	}
	r.snapshots[id] = snapshot
}

// AttachSession sets the editing session for a registered source and records
// its stale-guard snapshot (a missing file records nothing: its first save
// creates it). It wires sessions the registry cannot build
// itself: test doubles, and the mcpfile session from a later wave. The
// session path must match the source path.
func (r *Registry) AttachSession(id string, session Session) error {
	if session == nil {
		return fmt.Errorf("attaching session to %s: nil session", id)
	}
	for i := range r.sources {
		if r.sources[i].ID != id {
			continue
		}
		if session.Path() != r.sources[i].Path {
			return fmt.Errorf("attaching session to %s: session path %s does not match source path %s",
				id, session.Path(), r.sources[i].Path)
		}
		r.sources[i].Session = session
		r.remember(id, r.sources[i].Path)
		return nil
	}
	return fmt.Errorf("attaching session to %s: unknown source", id)
}

// SaveAll saves every dirty session in registry order, one file at a time.
// Each file gets its own atomic save and timestamped backup from its
// session. A stale or failing file is reported and skipped while the rest
// still save; clean files are never touched.
func (r *Registry) SaveAll() SaveResult {
	var out SaveResult
	for i := range r.sources {
		src := r.sources[i]
		if !src.Writable || src.Session == nil || len(src.Session.DirtyPaths()) == 0 {
			continue
		}
		out.Files = append(out.Files, r.saveOne(src))
	}
	return out
}

func (r *Registry) saveOne(src Source) FileResult {
	if r.isStale(src) {
		return FileResult{
			SourceID: src.ID,
			Path:     src.Path,
			Stale:    true,
			Err:      fmt.Errorf("saving %s: refusing stale file changed on disk since load", src.Path),
		}
	}
	backup, err := src.Session.Save()
	if err != nil {
		return FileResult{
			SourceID: src.ID,
			Path:     src.Path,
			Err:      fmt.Errorf("saving %s: %w", src.Path, err),
		}
	}
	r.remember(src.ID, src.Path)
	return FileResult{SourceID: src.ID, Path: src.Path, Backup: backup}
}

func (r *Registry) isStale(src Source) bool {
	want, ok := r.snapshots[src.ID]
	if !ok {
		r.remember(src.ID, src.Path)
		return false
	}
	return want != snapshotOf(src.Path)
}

// Reload re-reads every session from disk, discarding unsaved edits, and
// refreshes the stale-guard snapshots so a tampered file becomes saveable
// again. Failures are collected per file; successful files still reload.
func (r *Registry) Reload() error {
	var errs []error
	for i := range r.sources {
		src := r.sources[i]
		if src.Session == nil {
			continue
		}
		if err := src.Session.Reload(); err != nil {
			errs = append(errs, fmt.Errorf("reloading %s: %w", src.Path, err))
			continue
		}
		r.remember(src.ID, src.Path)
	}
	if len(errs) > 0 {
		return fmt.Errorf("reloading workspace: %w", errors.Join(errs...))
	}
	return nil
}
