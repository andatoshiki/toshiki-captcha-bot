package main

import (
	"testing"

	tele "gopkg.in/telebot.v3"
)

func TestIsUserInChatAdmins(t *testing.T) {
	t.Parallel()

	admins := []tele.ChatMember{
		{User: &tele.User{ID: 1001}},
		{User: &tele.User{ID: 1002}},
		{User: nil},
	}

	tests := []struct {
		name   string
		userID int64
		want   bool
	}{
		{
			name:   "zero id",
			userID: 0,
			want:   false,
		},
		{
			name:   "negative id",
			userID: -1,
			want:   false,
		},
		{
			name:   "admin id",
			userID: 1001,
			want:   true,
		},
		{
			name:   "non admin id",
			userID: 1003,
			want:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isUserInChatAdmins(tt.userID, admins); got != tt.want {
				t.Fatalf("isUserInChatAdmins() = %v, want %v", got, tt.want)
			}
		})
	}
}
