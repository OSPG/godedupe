package main

import "os/exec"

// Notification contains the title and the icon route to show
type Notification struct {
	title   string
	summary string
	icon    string
}

// ShowNotification show a notification desktop
func (notification *Notification) ShowNotification(show bool) {
	if show {
		exec.Command("notify-send", "-i",
			notification.icon, notification.title, notification.summary).Run()
	}
}
