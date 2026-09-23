package workflow

import "testing"

func TestValidateTransition(t *testing.T) {
	if err := ValidateTransition("queued", "running"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTransition("queued", "completed"); err == nil {
		t.Fatal("expected authority-bypassing transition to fail")
	}
}

func TestReplay(t *testing.T) {
	state, err := Replay("queued", []EventView{
		{Seq: 1, FromStatus: "queued", ToStatus: "running"},
		{Seq: 2, FromStatus: "running", ToStatus: "waiting_human"},
		{Seq: 3, FromStatus: "waiting_human", ToStatus: "running"},
		{Seq: 4, FromStatus: "running", ToStatus: "verifying"},
		{Seq: 5, FromStatus: "verifying", ToStatus: "completed"},
	})
	if err != nil || state != "completed" {
		t.Fatalf("state=%s err=%v", state, err)
	}
}
