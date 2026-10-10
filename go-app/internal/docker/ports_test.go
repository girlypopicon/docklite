package docker

import (
	"os"
	"testing"
)

func TestPickPortChoosesLowestFree(t *testing.T) {
	p, ok := pickPort(20000, 20005, map[int]bool{20000: true, 20001: true, 20003: true})
	if !ok || p != 20002 {
		t.Fatalf("got %d, %v; want 20002", p, ok)
	}
	if _, ok := pickPort(20000, 20001, map[int]bool{20000: true, 20001: true}); ok {
		t.Fatal("a full range must report no free port")
	}
}

func TestPortRangeParsing(t *testing.T) {
	cases := map[string][2]int{"": {20000, 29999}, "30000-30100": {30000, 30100}, " 25000 - 25010 ": {25000, 25010},
		"80-90": {20000, 29999}, "abc": {20000, 29999}, "40000-30000": {20000, 29999}, "1000-70000": {20000, 29999}}
	for in, want := range cases {
		t.Setenv("DOCKLITE_PORT_RANGE", in)
		if lo, hi := PortRange(); lo != want[0] || hi != want[1] {
			t.Errorf("%q: got %d-%d, want %d-%d", in, lo, hi, want[0], want[1])
		}
	}
	os.Unsetenv("DOCKLITE_PORT_RANGE")
}
