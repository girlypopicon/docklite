package docker

import "testing"

func TestLimitsValidate(t *testing.T) {
	host := HostResources{CPUs: 4, MemoryMB: 8192}
	good := []Limits{{}, {CPUs: 0.5}, {MemoryMB: 512}, {Pids: 100}, {CPUs: 4, MemoryMB: 8192, Pids: 4000}}
	for _, l := range good {
		if err := l.Validate(host); err != nil {
			t.Errorf("%+v should be fine: %v", l, err)
		}
	}
	bad := []Limits{{CPUs: -1}, {CPUs: 0.01}, {CPUs: 5}, {MemoryMB: 4}, {MemoryMB: 9000}, {Pids: 3}, {MemoryMB: -2}}
	for _, l := range bad {
		if err := l.Validate(host); err == nil {
			t.Errorf("%+v should be refused", l)
		}
	}
}

func TestLimitsResources(t *testing.T) {
	r := Limits{CPUs: 0.5, MemoryMB: 256, Pids: 200}.Resources()
	if r.NanoCPUs != 500_000_000 || r.Memory != 256<<20 || r.MemorySwap != r.Memory || r.PidsLimit == nil || *r.PidsLimit != 200 {
		t.Fatalf("unexpected: %+v", r)
	}
	if empty := (Limits{}).Resources(); empty.NanoCPUs != 0 || empty.Memory != 0 || empty.PidsLimit != nil {
		t.Fatalf("no limits must set nothing: %+v", empty)
	}
	// Removing a limit from a running container must say "unlimited" explicitly: 0 means "leave as is" to Docker.
	u := Limits{}.updateResources(HostResources{CPUs: 4, MemoryMB: 1024})
	if u.NanoCPUs != 4e9 || u.Memory != 1024<<20 || u.MemorySwap != -1 || u.PidsLimit == nil || *u.PidsLimit != -1 {
		t.Fatalf("unexpected unlimited update: %+v", u)
	}
}
