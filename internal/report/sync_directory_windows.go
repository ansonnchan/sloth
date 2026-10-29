package report

// Windows does not expose a standard-library equivalent of fsync on a
// directory handle. File contents are synced before the atomic publish.
func syncDirectory(string) error { return nil }
