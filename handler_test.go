package main

import "testing"

func TestIsNextCaptchaAnswer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		status       JoinStatus
		answer       string
		wantOK       bool
		wantExpected string
	}{
		{
			name: "first step correct",
			status: JoinStatus{
				CaptchaAnswer: []string{"u1", "u2", "u3", "u4"},
				SolvedCaptcha: 0,
			},
			answer:       "u1",
			wantOK:       true,
			wantExpected: "u1",
		},
		{
			name: "first step wrong",
			status: JoinStatus{
				CaptchaAnswer: []string{"u1", "u2", "u3", "u4"},
				SolvedCaptcha: 0,
			},
			answer:       "u2",
			wantOK:       false,
			wantExpected: "u1",
		},
		{
			name: "duplicate previous tap is wrong",
			status: JoinStatus{
				CaptchaAnswer: []string{"u1", "u2", "u3", "u4"},
				SolvedCaptcha: 1,
			},
			answer:       "u1",
			wantOK:       false,
			wantExpected: "u2",
		},
		{
			name: "next step correct",
			status: JoinStatus{
				CaptchaAnswer: []string{"u1", "u2", "u3", "u4"},
				SolvedCaptcha: 2,
			},
			answer:       "u3",
			wantOK:       true,
			wantExpected: "u3",
		},
		{
			name: "out of range solved index",
			status: JoinStatus{
				CaptchaAnswer: []string{"u1", "u2", "u3", "u4"},
				SolvedCaptcha: 4,
			},
			answer:       "u4",
			wantOK:       false,
			wantExpected: "",
		},
		{
			name: "empty answers",
			status: JoinStatus{
				CaptchaAnswer: []string{},
				SolvedCaptcha: 0,
			},
			answer:       "u1",
			wantOK:       false,
			wantExpected: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotOK, gotExpected := isNextCaptchaAnswer(tt.status, tt.answer)
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotExpected != tt.wantExpected {
				t.Fatalf("expected = %q, want %q", gotExpected, tt.wantExpected)
			}
		})
	}
}
