package hf

import "testing"

func TestParseRef(t *testing.T) {
	cases := []struct {
		in, rev           string
		ns, name, wantRev string
		wantErr           bool
	}{
		{"hf:Qwen/Qwen2.5-7B-Instruct", "", "Qwen", "Qwen2.5-7B-Instruct", "main", false},
		{"mistralai/Mistral-7B-Instruct-v0.3", "", "mistralai", "Mistral-7B-Instruct-v0.3", "main", false},
		{"hf:Qwen/Qwen2.5-7B-Instruct", "dev", "Qwen", "Qwen2.5-7B-Instruct", "dev", false},
		{"hf:Qwen/Qwen2.5@v1", "", "Qwen", "Qwen2.5", "v1", false},
		{"bad-id", "", "", "", "", true},
		{"", "", "", "", "", true},
	}
	for _, c := range cases {
		ref, err := ParseRef(c.in, c.rev)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseRef(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRef(%q): unexpected error %v", c.in, err)
			continue
		}
		if ref.Namespace != c.ns || ref.Name != c.name || ref.Revision != c.wantRev {
			t.Errorf("ParseRef(%q) = %+v, want ns=%s name=%s rev=%s", c.in, ref, c.ns, c.name, c.wantRev)
		}
	}
}

func TestEstimateParamsFromName(t *testing.T) {
	cases := []struct {
		name string
		want int64
	}{
		{"Qwen2.5-7B-Instruct", 7_000_000_000},
		{"Mistral-7B-Instruct-v0.3", 7_000_000_000},
		{"phi-3-mini", 0},
		{"opt-350m", 350_000_000},
	}
	for _, c := range cases {
		if got := EstimateParamsFromName(c.name); got != c.want {
			t.Errorf("EstimateParamsFromName(%q) = %d, want %d", c.name, got, c.want)
		}
	}
}
