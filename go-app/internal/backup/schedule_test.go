package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"docklite-agent/internal/store"
	"docklite-agent/internal/testhelpers"
)

func tm(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, time.UTC)
	if err != nil {
		panic(err)
	}
	return t
}

func TestParseSchedule(t *testing.T) {
	good := map[string]Schedule{
		"hourly":             {Kind: "hourly"},
		"daily":              {Kind: "daily"},
		"6":                  {Kind: "hours", Hours: 6},
		"daily@03:00":        {Kind: "daily", At: "03:00"},
		"daily@03:00;keep=7": {Kind: "daily", At: "03:00", KeepLast: 7},
		"weekly@sun@04:30":   {Kind: "weekly", At: "04:30", HasWeekly: true, Weekday: time.Sunday},
		"daily;keep=3":       {Kind: "daily", KeepLast: 3},
		"every-6-hours":      {Kind: "hours", Hours: 6},
	}
	for in, want := range good {
		got, err := ParseSchedule(in)
		if err != nil || got != want {
			t.Errorf("%q: got %+v, %v; want %+v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "sometimes", "daily@25:00", "daily@3am", "hourly@03:00", "weekly@funday@03:00", "weekly@sun", "daily;keep=-1", "daily;keep=x", "0", "-5", "daily@03:00@04:00", "every-3-days@03:00", "monthly@03:00"} {
		if _, err := ParseSchedule(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestScheduleDueAtTimeOfDay(t *testing.T) {
	s, _ := ParseSchedule("daily@03:00")
	// ran yesterday at 3:00:05; it's 02:59 today: not yet; 03:00 today: due
	last := tm("2026-10-11 03:00")
	if s.Due(last, tm("2026-10-12 02:59")) {
		t.Fatal("not due before today's slot")
	}
	if !s.Due(last, tm("2026-10-12 03:00")) {
		t.Fatal("due at the slot")
	}
	// ran at today's slot: not due again that day
	if s.Due(tm("2026-10-12 03:00"), tm("2026-10-12 23:59")) {
		t.Fatal("must not run twice for one slot")
	}
	// the server was off through 3 am and comes back at 09:00: it still takes today's backup
	if !s.Due(tm("2026-10-11 03:00"), tm("2026-10-12 09:00")) {
		t.Fatal("a missed slot should be caught up")
	}
	if !s.Due(time.Time{}, tm("2026-10-12 09:00")) {
		t.Fatal("never run before: due")
	}
}

func TestScheduleNext(t *testing.T) {
	s, _ := ParseSchedule("daily@03:00")
	if got := s.Next(tm("2026-10-12 03:00"), tm("2026-10-12 03:05")); got != tm("2026-10-13 03:00") {
		t.Fatalf("got %v", got)
	}
	if got := s.Next(time.Time{}, tm("2026-10-12 01:00")); got != tm("2026-10-12 03:00") {
		t.Fatalf("before today's slot the next is today's: %v", got)
	}
	w, _ := ParseSchedule("weekly@sun@03:00")
	// 2026-10-12 is a Monday; next Sunday is the 18th
	if got := w.Next(time.Time{}, tm("2026-10-12 10:00")); got != tm("2026-10-18 03:00") {
		t.Fatalf("got %v", got)
	}
	if !w.Due(tm("2026-10-04 03:00"), tm("2026-10-11 03:00")) { // 11th is a Sunday
		t.Fatal("a week later on Sunday 03:00 it is due")
	}
	if w.Due(tm("2026-10-11 03:00"), tm("2026-10-17 12:00")) {
		t.Fatal("not due mid-week")
	}
	old, _ := ParseSchedule("daily")
	if old.Due(tm("2026-10-12 00:00"), tm("2026-10-12 23:00")) || !old.Due(tm("2026-10-12 00:00"), tm("2026-10-13 00:00")) {
		t.Fatal("old-style daily still means every 24 hours")
	}
}

func TestDescribe(t *testing.T) {
	for in, want := range map[string]string{
		"daily@03:00;keep=7": "every day at 03:00, keeping the last 7",
		"weekly@sun@03:00":   "every Sunday at 03:00",
		"hourly":             "every hour",
		"12":                 "every 12 hours",
	} {
		s, _ := ParseSchedule(in)
		if got := s.Describe(); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestPruneKeepLast(t *testing.T) {
	db := testhelpers.TestStoreWithTables(t)
	s := &store.SQLiteStore{DB: db, Path: ":memory:"}
	dir := t.TempDir()
	dest, err := s.CreateBackupDestination("Local", "local", "{}", 1)
	if err != nil {
		t.Fatal(err)
	}
	mk := func(i int, status string) string {
		p := filepath.Join(dir, fmt.Sprintf("site-a-%d.tar.gz", i))
		_ = os.WriteFile(p, []byte("x"), 0o644)
		id, err := s.CreateBackup(store.BackupRecord{Destination: dest, TargetType: "site", TargetID: 1, Status: "in_progress"})
		if err != nil {
			t.Fatal(err)
		}
		size := int64(1)
		_ = s.UpdateBackupStatus(id, status, nil, &size, &p)
		_, _ = db.Exec(`UPDATE backups SET created_at = ? WHERE id = ?`, fmt.Sprintf("2026-10-%02d 03:00:00", i), id)
		return p
	}
	var paths []string
	for i := 1; i <= 5; i++ {
		paths = append(paths, mk(i, "success"))
	}
	failed := mk(6, "failed") // newest, but failed: doesn't count and isn't removed
	other := filepath.Join(dir, "other.tar.gz")
	_ = os.WriteFile(other, []byte("x"), 0o644)
	oid, _ := s.CreateBackup(store.BackupRecord{Destination: dest, TargetType: "site", TargetID: 2, Status: "in_progress"})
	sz := int64(1)
	_ = s.UpdateBackupStatus(oid, "success", nil, &sz, &other)

	pruneKeepLast(s, "site", 1, 3)

	for i, p := range paths {
		_, err := os.Stat(p)
		if i < 2 && err == nil {
			t.Errorf("backup %d should have been removed", i+1)
		}
		if i >= 2 && err != nil {
			t.Errorf("backup %d should have been kept", i+1)
		}
	}
	if _, err := os.Stat(failed); err != nil {
		t.Error("a failed backup is not pruned by count")
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("another site's backups must never be touched")
	}
}
