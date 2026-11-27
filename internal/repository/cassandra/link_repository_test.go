package cassandra

import (
	"testing"
	"time"
	_ "time/tzdata"
)

func TestAnalyticsWindow(t *testing.T) {
	saoPaulo, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("failed to load America/Sao_Paulo: %v", err)
	}

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("failed to load Asia/Tokyo: %v", err)
	}

	t.Run("America/Sao_Paulo at 21:05 local time (next UTC day)", func(t *testing.T) {
		// 2026-11-26 21:05:17 -03:00 == 2026-11-27 00:05:17 UTC
		nowLocal := time.Date(2026, 11, 26, 21, 5, 17, 0, saoPaulo)
		startUTC, endUTC, buckets := analyticsWindow(nowLocal, 7, saoPaulo)

		wantStart := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)
		wantEnd := time.Date(2026, 11, 27, 3, 0, 0, 0, time.UTC)
		if !startUTC.Equal(wantStart) {
			t.Fatalf("startUTC = %v, want %v", startUTC, wantStart)
		}
		if !endUTC.Equal(wantEnd) {
			t.Fatalf("endUTC = %v, want %v", endUTC, wantEnd)
		}

		wantBuckets := []string{
			"2026-11-20",
			"2026-11-21",
			"2026-11-22",
			"2026-11-23",
			"2026-11-24",
			"2026-11-25",
			"2026-11-26",
			"2026-11-27",
		}
		if len(buckets) != len(wantBuckets) {
			t.Fatalf("len(buckets) = %d (%v), want %d (%v)", len(buckets), buckets, len(wantBuckets), wantBuckets)
		}
		for i, want := range wantBuckets {
			if buckets[i] != want {
				t.Errorf("buckets[%d] = %s, want %s", i, buckets[i], want)
			}
		}

		// Click at 2026-11-27 00:05:17 UTC must fall inside [startUTC, endUTC) and map to 2026-11-26 in Sao Paulo.
		clickUTC := time.Date(2026, 11, 27, 0, 5, 17, 0, time.UTC)
		if clickUTC.Before(startUTC) || !clickUTC.Before(endUTC) {
			t.Fatalf("expected %v to be within [%v, %v)", clickUTC, startUTC, endUTC)
		}
		if gotDate := clickUTC.In(saoPaulo).Format("2006-01-02"); gotDate != "2026-11-26" {
			t.Fatalf("local date = %s, want 2026-11-26", gotDate)
		}
	})

	t.Run("UTC exact 7-day window", func(t *testing.T) {
		nowUTC := time.Date(2026, 11, 26, 15, 0, 0, 0, time.UTC)
		startUTC, endUTC, buckets := analyticsWindow(nowUTC, 7, time.UTC)

		wantStart := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
		wantEnd := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
		if !startUTC.Equal(wantStart) || !endUTC.Equal(wantEnd) {
			t.Fatalf("got [%v, %v), want [%v, %v)", startUTC, endUTC, wantStart, wantEnd)
		}
		if len(buckets) != 7 {
			t.Fatalf("len(buckets) = %d (%v), want 7", len(buckets), buckets)
		}
	})

	t.Run("Asia/Tokyo positive offset", func(t *testing.T) {
		// 2026-11-26 02:00:00 +09:00 == 2026-11-25 17:00:00 UTC
		nowLocal := time.Date(2026, 11, 26, 2, 0, 0, 0, tokyo)
		startUTC, endUTC, buckets := analyticsWindow(nowLocal, 1, tokyo)

		wantStart := time.Date(2026, 11, 25, 15, 0, 0, 0, time.UTC)
		wantEnd := time.Date(2026, 11, 26, 15, 0, 0, 0, time.UTC)
		if !startUTC.Equal(wantStart) || !endUTC.Equal(wantEnd) {
			t.Fatalf("got [%v, %v), want [%v, %v)", startUTC, endUTC, wantStart, wantEnd)
		}
		if len(buckets) != 2 || buckets[0] != "2026-11-25" || buckets[1] != "2026-11-26" {
			t.Fatalf("buckets = %v, want [2026-11-25 2026-11-26]", buckets)
		}
	})
}
