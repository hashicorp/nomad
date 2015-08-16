package nomad

import (
	"reflect"
	"testing"
	"time"
)

func TestTimeTable(t *testing.T) {
	tt := NewTimeTable(time.Second, time.Minute)

	index := tt.NearestIndex(time.Now())
	if index != 0 {
		t.Fatalf("bad: %v", index)
	}

	when := tt.NearestTime(1000)
	if !when.IsZero() {
		t.Fatalf("bad: %v", when)
	}

	// Witness some data
	start := time.Now()
	plusOne := start.Add(time.Minute)
	plusTwo := start.Add(2 * time.Minute)
	plusFive := start.Add(5 * time.Minute)
	plusThirty := start.Add(30 * time.Minute)
	plusHour := start.Add(60 * time.Minute)
	plusHourHalf := start.Add(90 * time.Minute)

	tt.Witness(2, start)
	tt.Witness(2, start)

	tt.Witness(10, plusOne)
	tt.Witness(10, plusOne)

	tt.Witness(20, plusTwo)
	tt.Witness(20, plusTwo)

	tt.Witness(30, plusFive)
	tt.Witness(30, plusFive)

	tt.Witness(40, plusThirty)
	tt.Witness(40, plusThirty)

	tt.Witness(50, plusHour)
	tt.Witness(50, plusHour)

	type tcase struct {
		when        time.Time
		expectIndex uint64

		index      uint64
		expectWhen time.Time
	}
	cases := []tcase{
		// Exact match
		{start, 2, 2, start},
		{plusOne, 10, 10, plusOne},
		{plusHour, 50, 50, plusHour},

		// Before the newest entry
		{plusHourHalf, 50, 51, plusHour},

		// After the oldest entry
		{time.Time{}, 0, 1, time.Time{}},

		// Mid range
		{start.Add(3 * time.Minute), 20, 25, plusTwo},
	}

	for _, tc := range cases {
		index := tt.NearestIndex(tc.when)
		if index != tc.expectIndex {
			t.Fatalf("bad: %v %v", index, tc.expectIndex)
		}

		when := tt.NearestTime(tc.index)
		if when != tc.expectWhen {
			t.Fatalf("bad: for %d %v %v", tc.index, when, tc.expectWhen)
		}
	}
}

func TestTimeTable_SerializeDeserialize(t *testing.T) {
	tt := NewTimeTable(time.Second, time.Minute)

	// Witness some data
	start := time.Now()
	plusOne := start.Add(time.Minute)
	plusTwo := start.Add(2 * time.Minute)
	plusFive := start.Add(5 * time.Minute)
	plusThirty := start.Add(30 * time.Minute)
	plusHour := start.Add(60 * time.Minute)

	tt.Witness(2, start)
	tt.Witness(10, plusOne)
	tt.Witness(20, plusTwo)
	tt.Witness(30, plusFive)
	tt.Witness(40, plusThirty)
	tt.Witness(50, plusHour)

	buf, err := tt.Serialize()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	tt2 := NewTimeTable(time.Second, time.Minute)
	err = tt2.Deserialize(buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(tt.table, tt2.table) {
		t.Fatalf("bad: %#v %#v", tt, tt2)
	}
}
