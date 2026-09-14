package topology

import "testing"

func TestStablePartitionsAndOwnership(t *testing.T) {
	a, _ := Parse("pu-1=http://one:8080,pu-2=http://two:8080,pu-3=http://three:8080", 128, "1")
	b, _ := Parse("pu-1=http://one:8080", 128, "2")
	// SHA256("abc") starts ba7816bf8f01cfea: low seven bits are 106.
	if a.Partition("abc") != 106 {
		t.Fatalf("hash contract changed: %d", a.Partition("abc"))
	}
	for _, key := range []string{"abc", "tenant-1", "Москва"} {
		p := a.Partition(key)
		if p != b.Partition(key) {
			t.Fatal("partition depends on topology")
		}
		if a.Owner(p).ID != a.Nodes[p%3].ID {
			t.Fatal("wrong owner")
		}
	}
}
func TestRejectInvalidTopology(t *testing.T) {
	for _, s := range []string{"", "a=http://a,a=http://b", "a=file:///tmp", "a=http://a/path"} {
		if _, e := Parse(s, 128, "1"); e == nil {
			t.Fatalf("accepted %q", s)
		}
	}
}
