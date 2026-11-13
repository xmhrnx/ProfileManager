package proc

import "testing"

func TestMatches(t *testing.T) {
	cases := []struct {
		running string
		names   []string
		want    bool
	}{
		{"Slack.exe", []string{"slack"}, true},
		{"/Applications/Slack.app/Contents/MacOS/Slack", []string{"Slack"}, true},
		{"telegram-deskto", []string{"telegram-desktop"}, true}, // 15-char comm
		{"slackd", []string{"slack"}, false},
		{"claude", []string{"claude"}, true},
	}
	for _, c := range cases {
		if got := Matches(c.running, c.names); got != c.want {
			t.Errorf("Matches(%q, %v) = %v", c.running, c.names, got)
		}
	}
}
