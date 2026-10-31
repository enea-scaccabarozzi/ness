package sync

import (
	"bytes"
	"strings"
	"testing"
)

func TestBlockMarkers(t *testing.T) {
	// covers: R-SYNC-040, R-SYNC-042
	out, err := setBlock([]byte("# Title\n"), markdownStyle, "core/router", []byte("hello\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := "# Title\n\n<!-- ness:begin core/router -->\nhello\n<!-- ness:end core/router -->\n"
	if string(out) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
	out, err = setBlock(nil, styleFor(".gitignore"), "core/ignore", []byte(".worktrees/\n*.ness-new"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "# ness:begin core/ignore\n.worktrees/\n*.ness-new\n# ness:end core/ignore\n" {
		t.Fatalf("gitignore block %q", out)
	}
}

func TestBlockMovedIsUpdatedInPlace(t *testing.T) {
	// covers: R-SYNC-041, R-SYNC-043
	in := "<!-- ness:begin core/router -->\nold\n<!-- ness:end core/router -->\n# Mine\ntext\n"
	got, ok, err := getBlock([]byte(in), markdownStyle, "core/router")
	if err != nil || !ok || string(got) != "old\n" {
		t.Fatalf("getBlock = %q %v %v", got, ok, err)
	}
	out, err := setBlock([]byte(in), markdownStyle, "core/router", []byte("new\n"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != strings.Replace(in, "old", "new", 1) {
		t.Fatalf("got %q", out)
	}
}

func TestRemoveBlock(t *testing.T) {
	in := "a\n<!-- ness:begin x/y -->\nz\n<!-- ness:end x/y -->\nb\n"
	out, err := removeBlock([]byte(in), markdownStyle, "x/y")
	if err != nil || string(out) != "a\nb\n" {
		t.Fatalf("removeBlock = %q %v", out, err)
	}
	added, err := setBlock([]byte("intro\n"), markdownStyle, "x/y", []byte("z\n"))
	if err != nil {
		t.Fatal(err)
	}
	if out, err := removeBlock(added, markdownStyle, "x/y"); err != nil || string(out) != "intro\n" {
		t.Fatalf("append then remove = %q %v", out, err)
	}
}

func TestBlocksMalformed(t *testing.T) {
	// covers: R-SYNC-046
	cases := map[string]string{
		"no end":     "a\n<!-- ness:begin x/y -->\nb\n",
		"nested":     "<!-- ness:begin x/y -->\n<!-- ness:begin x/z -->\n<!-- ness:end x/z -->\n<!-- ness:end x/y -->\n",
		"duplicate":  "<!-- ness:begin x/y -->\n<!-- ness:end x/y -->\n<!-- ness:begin x/y -->\n<!-- ness:end x/y -->\n",
		"stray end":  "<!-- ness:end x/y -->\n",
		"bad name":   "<!-- ness:begin Router -->\n<!-- ness:end Router -->\n",
		"mismatched": "<!-- ness:begin x/y -->\n<!-- ness:end x/z -->\n",
	}
	for name, in := range cases {
		if _, err := parseBlocks([]byte(in), markdownStyle); err == nil || !strings.Contains(err.Error(), "line ") {
			t.Errorf("%s: err = %v, want an error naming the line", name, err)
		}
	}
}

func TestBlocksCRLF(t *testing.T) {
	in := "a\r\n<!-- ness:begin x/y -->\r\nold\r\n<!-- ness:end x/y -->\r\nb\r\n"
	got, ok, err := getBlock([]byte(in), markdownStyle, "x/y")
	if err != nil || !ok || string(got) != "old\r\n" {
		t.Fatalf("getBlock = %q %v %v", got, ok, err)
	}
}

func FuzzBlocks(f *testing.F) {
	// covers: R-SYNC-044, R-SYNC-047
	f.Add([]byte("intro\n<!-- ness:begin core/router -->\nold\n<!-- ness:end core/router -->\noutro\n"), []byte("new\n"))
	f.Add([]byte("<!-- ness:begin a/b -->\n"), []byte("x"))
	f.Add([]byte("no blocks"), []byte(""))
	f.Fuzz(func(t *testing.T, data, inner []byte) {
		if bytes.Contains(inner, []byte("ness:")) {
			return
		}
		blocks, err := parseBlocks(data, markdownStyle)
		if err != nil {
			return
		}
		out, err := setBlock(data, markdownStyle, "core/router", inner)
		if err != nil {
			t.Fatal(err)
		}
		got, ok, err := getBlock(out, markdownStyle, "core/router")
		want := normInner(inner)
		if err != nil || !ok || !bytes.Equal(got, want) {
			t.Fatalf("round trip: got %q %v %v, want %q", got, ok, err, want)
		}
		for _, b := range blocks {
			if b.name == "core/router" {
				if !bytes.Equal(out[:b.inner], data[:b.inner]) || !bytes.HasSuffix(out, data[b.innerEnd:]) {
					t.Fatal("bytes outside the block changed")
				}
				return
			}
		}
		if !bytes.HasPrefix(out, data) {
			t.Fatal("appending changed existing bytes")
		}
	})
}
