package logs

import "testing"

func TestRingBounded(t *testing.T) {
	r := NewRing(8)
	_, _ = r.Write([]byte("123456"))
	_, _ = r.Write([]byte("789012"))
	// Retains the most recent 8 bytes of "123456789012".
	if got := r.String(); got != "56789012" {
		t.Errorf("unexpected ring contents %q", got)
	}
}

func TestBusReplayAndLive(t *testing.T) {
	b := NewBus()
	b.Publish(Event{Step: "a", Status: "ok"})

	hist, ch, cancel := b.Subscribe()
	defer cancel()
	if len(hist) != 1 || hist[0].Step != "a" {
		t.Fatalf("expected replay of 1 event, got %v", hist)
	}

	b.Publish(Event{Step: "b", Status: "ok"})
	select {
	case e := <-ch:
		if e.Step != "b" {
			t.Errorf("got live event %v", e)
		}
	default:
		t.Error("expected live event on channel")
	}

	b.Close()
	if _, ok := <-ch; ok {
		t.Error("expected channel closed after bus close")
	}
}
