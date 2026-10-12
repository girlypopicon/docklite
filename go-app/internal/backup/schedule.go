package backup

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule is what a backup job's "frequency" text means. The text keeps working for every value earlier versions
// wrote (hourly, daily, every-3-days, weekly, monthly, or a number of hours) and adds a time of day and a number of
// copies to keep, so no database change is needed:
//
//	daily@03:00              every day at 3 am (server time)
//	weekly@sun@03:00         every Sunday at 3 am
//	daily@03:00;keep=7       ...and keep only the 7 newest backups of each site
//	daily                    (old style) every 24 hours from the last run
type Schedule struct {
	Kind      string // "hourly", "daily", "weekly", "every-3-days", "monthly", "hours"
	Hours     int    // for Kind "hours"
	At        string // "HH:MM" when a time of day was chosen, else ""
	Weekday   time.Weekday
	HasWeekly bool
	KeepLast  int // 0 = no limit by count
}

var weekdayNames = map[string]time.Weekday{"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday}

// ParseSchedule reads a frequency text. Unknown text is an error, so a typo never silently means "never".
func ParseSchedule(freq string) (Schedule, error) {
	var s Schedule
	freq = strings.TrimSpace(freq)
	main, opts, _ := strings.Cut(freq, ";")
	for _, o := range strings.Split(opts, ";") {
		if k, v, ok := strings.Cut(strings.TrimSpace(o), "="); ok && k == "keep" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 || n > 1000 {
				return s, fmt.Errorf("keep must be a number from 0 to 1000")
			}
			s.KeepLast = n
		}
	}
	parts := strings.Split(main, "@")
	s.Kind = parts[0]
	switch s.Kind {
	case "hourly", "daily", "every-3-days", "monthly":
		if len(parts) > 2 {
			return s, fmt.Errorf("unrecognised schedule %q", freq)
		}
		if len(parts) == 2 {
			// a time of day only makes sense for daily (and weekly, below); the others count from the last run
			if s.Kind != "daily" || !validClock(parts[1]) {
				return s, fmt.Errorf("unrecognised schedule %q", freq)
			}
			s.At = parts[1]
		}
	case "weekly":
		switch len(parts) {
		case 1:
		case 3:
			wd, ok := weekdayNames[strings.ToLower(parts[1])]
			if !ok || !validClock(parts[2]) {
				return s, fmt.Errorf("unrecognised schedule %q", freq)
			}
			s.Weekday, s.HasWeekly, s.At = wd, true, parts[2]
		default:
			return s, fmt.Errorf("unrecognised schedule %q", freq)
		}
	default:
		kind := strings.TrimSuffix(strings.TrimPrefix(s.Kind, "every-"), "-hours") // older form: every-6-hours
		h, err := strconv.Atoi(kind)
		if err != nil || h <= 0 {
			return s, fmt.Errorf("unrecognised schedule %q", freq)
		}
		s.Kind, s.Hours = "hours", h
	}
	return s, nil
}

func validClock(v string) bool {
	t, err := time.Parse("15:04", v)
	return err == nil && t.Format("15:04") == v
}

// interval is the gap between runs for the schedules that count from the last run.
func (s Schedule) interval() time.Duration {
	switch s.Kind {
	case "hourly":
		return time.Hour
	case "daily":
		return 24 * time.Hour
	case "every-3-days":
		return 72 * time.Hour
	case "weekly":
		return 7 * 24 * time.Hour
	case "monthly":
		return 30 * 24 * time.Hour
	}
	return time.Duration(s.Hours) * time.Hour
}

// slotAtOrBefore is the most recent scheduled time that is not after now (only for schedules with a time of day).
func (s Schedule) slotAtOrBefore(now time.Time) time.Time {
	hh, _ := strconv.Atoi(s.At[:2])
	mm, _ := strconv.Atoi(s.At[3:])
	slot := time.Date(now.Year(), now.Month(), now.Day(), hh, mm, 0, 0, now.Location())
	if s.HasWeekly {
		for slot.Weekday() != s.Weekday {
			slot = slot.AddDate(0, 0, -1)
		}
	}
	step := 1
	if s.HasWeekly {
		step = 7
	}
	for slot.After(now) {
		slot = slot.AddDate(0, 0, -step)
	}
	return slot
}

// Due says whether a job should run now. last is when it last ran (zero if never). A job with a time of day runs once
// the first time its slot has passed, so a server that was off at 3 am still takes that day's backup when it comes back.
func (s Schedule) Due(last, now time.Time) bool {
	if last.IsZero() {
		return true
	}
	if s.At == "" {
		return now.Sub(last) >= s.interval()
	}
	return last.Before(s.slotAtOrBefore(now))
}

// Next is when the job will next be due, after now.
func (s Schedule) Next(last, now time.Time) time.Time {
	if s.At == "" {
		base := last
		if base.IsZero() || base.After(now) {
			base = now
		}
		return base.Add(s.interval())
	}
	slot := s.slotAtOrBefore(now)
	for !slot.After(now) {
		slot = slot.AddDate(0, 0, map[bool]int{true: 7, false: 1}[s.HasWeekly])
	}
	return slot
}

// Describe puts a schedule in plain words for the dashboard and the terminal.
func (s Schedule) Describe() string {
	var when string
	switch {
	case s.Kind == "hours":
		when = fmt.Sprintf("every %d hours", s.Hours)
	case s.Kind == "hourly":
		when = "every hour"
	case s.HasWeekly:
		when = fmt.Sprintf("every %s at %s", s.Weekday, s.At)
	case s.Kind == "daily" && s.At != "":
		when = "every day at " + s.At
	case s.Kind == "daily":
		when = "every day"
	case s.Kind == "weekly":
		when = "every week"
	case s.Kind == "every-3-days":
		when = "every 3 days"
	case s.Kind == "monthly":
		when = "every month"
	}
	if s.At != "" && !s.HasWeekly && s.Kind != "daily" {
		when += " at " + s.At
	}
	if s.KeepLast > 0 {
		when += fmt.Sprintf(", keeping the last %d", s.KeepLast)
	}
	return when
}
