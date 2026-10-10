package handlers

import (
	"testing"

	"docklite-agent/internal/testhelpers"
)

func TestParseUpstreamBlocks(t *testing.T) {
	out := "/etc/nginx/sites-available/blog.example.com\t blog.example.com www.Blog.example.com\t 32001\n" +
		"/etc/nginx/conf.d/x.conf\t evrqr.example.com\t 32002 32099\n" +
		"\n" +
		"garbage line\n" +
		"/etc/nginx/sites-available/none\t only.example.com\t \n"
	blocks := parseUpstreamBlocks(out)
	testhelpers.AssertEqual(t, len(blocks), 3)
	testhelpers.AssertEqual(t, blocks[0].Names[1], "www.blog.example.com") // lower-cased
	testhelpers.AssertEqual(t, blocks[0].Ports[0], 32001)
	testhelpers.AssertEqual(t, len(blocks[1].Ports), 2)
	testhelpers.AssertEqual(t, len(blocks[2].Ports), 0)
}

func TestPlanUpstreamFixes(t *testing.T) {
	blocks := []upstreamBlock{
		{File: "/etc/nginx/sites-available/blog", Names: []string{"blog.example.com", "www.blog.example.com"}, Ports: []int{32001}},
		{File: "/etc/nginx/conf.d/evrqr.conf", Names: []string{"evrqr.example.com"}, Ports: []int{32002}},
		{File: "/etc/nginx/sites-available/docklite-sites", Names: []string{"a.example.com"}, Ports: []int{32003}},
		{File: "/etc/nginx/sites-available/docklite-sites", Names: []string{"a.example.com"}, Ports: []int{32003}}, // its https twin
		{File: "/etc/nginx/sites-available/docklite-sites", Names: []string{"split.example.com"}, Ports: []int{32004, 32044}},
		{File: "/etc/nginx/sites-available/docklite-sites", Names: []string{"fine.example.com"}, Ports: []int{40000}},
		{File: "/etc/nginx/sites-available/other", Names: []string{"unrelated.example.org"}, Ports: []int{1234}},
	}
	desired := map[string]int{
		"blog.example.com": 40001, "evrqr.example.com": 40002, "a.example.com": 40003,
		"split.example.com": 40004, "fine.example.com": 40000, "nginxless.example.com": 40005,
	}
	fixes, seen := planUpstreamFixes(blocks, desired, nil)

	got := map[string]upstreamFix{}
	for _, f := range fixes {
		got[f.Domain] = f
	}
	testhelpers.AssertEqual(t, got["blog.example.com"].New, 40001)                            // matched through the stored name
	testhelpers.AssertEqual(t, got["evrqr.example.com"].File, "/etc/nginx/conf.d/evrqr.conf") // conf.d is covered
	testhelpers.AssertEqual(t, got["a.example.com"].Old, 32003)
	_, hasSplit := got["split.example.com"]
	testhelpers.AssertFalse(t, hasSplit, "a block with two different upstreams must be left alone")
	_, hasFine := got["fine.example.com"]
	testhelpers.AssertFalse(t, hasFine, "a correct block needs no change")
	testhelpers.AssertEqual(t, len(fixes), 3) // blog, evrqr, a (once, not once per twin block)

	testhelpers.AssertTrue(t, seen["blog.example.com"], "seen")
	testhelpers.AssertFalse(t, seen["nginxless.example.com"], "a site with no nginx block is reported as missing")
}

func TestPlanNeverTouchesWhatItWasNotAskedTo(t *testing.T) {
	blocks := []upstreamBlock{{File: "/etc/nginx/conf.d/a", Names: []string{"mine.example.com"}, Ports: []int{3000}}}
	fixes, _ := planUpstreamFixes(blocks, map[string]int{"someone-else.example.com": 40000}, nil)
	testhelpers.AssertEqual(t, len(fixes), 0)
}

func TestAliasBlocksFollowTheirContainer(t *testing.T) {
	// docklite.net's nginx block proxies to the container that serves docklite.org: no name match, but the port moved.
	blocks := []upstreamBlock{
		{File: "/etc/nginx/conf.d/docklite-net.conf", Names: []string{"docklite.net", "www.docklite.net"}, Ports: []int{32775}},
		{File: "/etc/nginx/conf.d/leave-me.conf", Names: []string{"static.example.org"}, Ports: []int{8080}},
		{File: "/etc/nginx/conf.d/two.conf", Names: []string{"two.example.org"}, Ports: []int{32775, 32776}},
	}
	sites := []sitePort{{Name: "docklite-site-docklite-org", Domain: "docklite.org", Port: 40010}}
	moved := movedPorts(map[string]int{"docklite-site-docklite-org": 32775}, sites)
	testhelpers.AssertEqual(t, moved[32775], 40010)

	fixes, _ := planUpstreamFixes(blocks, desiredUpstreams(sites), moved)
	testhelpers.AssertEqual(t, len(fixes), 1)
	testhelpers.AssertEqual(t, fixes[0].Domain, "docklite.net")
	testhelpers.AssertEqual(t, fixes[0].New, 40010)
}

func TestMovedPortsIgnoresPortsStillInUse(t *testing.T) {
	// the old port was reused by a different running container, so it is not abandoned and must not be "followed"
	sites := []sitePort{{Name: "a", Domain: "a.example.com", Port: 40001}, {Name: "b", Domain: "b.example.com", Port: 32775}}
	moved := movedPorts(map[string]int{"a": 32775}, sites)
	testhelpers.AssertEqual(t, len(moved), 0)
	// unchanged containers don't move; containers not running now are ignored
	moved = movedPorts(map[string]int{"b": 32775, "gone": 5000}, sites)
	testhelpers.AssertEqual(t, len(moved), 0)
}

func TestDuplicateDomainContainersAreSkipped(t *testing.T) {
	d := desiredUpstreams([]sitePort{{Name: "x1", Domain: "dup.example.com", Port: 1}, {Name: "x2", Domain: "dup.example.com", Port: 2}, {Name: "y", Domain: "ok.example.com", Port: 3}})
	_, dup := d["dup.example.com"]
	testhelpers.AssertFalse(t, dup, "ambiguous domain")
	testhelpers.AssertEqual(t, d["ok.example.com"], 3)
}
