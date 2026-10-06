package workspace

// StaleSource names a registered source whose file changed on disk since
// the registry last saw it consistent (at load, attach, save, or reload).
// The save confirm lists these as blocked: their bytes are untouched and
// their dirty set is preserved until a reload.
type StaleSource struct {
	SourceID string
	Path     string
}

// Stale reports every registered source with an attached session whose file
// changed on disk since the last consistent snapshot. Sources without a
// snapshot yet (never loaded, attached, saved, or reloaded) are treated as
// fresh so the query stays pure: it never records snapshots itself.
func (r *Registry) Stale() []StaleSource {
	var out []StaleSource
	for _, src := range r.sources {
		if src.Session == nil {
			continue
		}
		want, ok := r.snapshots[src.ID]
		if !ok {
			continue
		}
		if want != snapshotOf(src.Path) {
			out = append(out, StaleSource{SourceID: src.ID, Path: src.Path})
		}
	}
	return out
}
