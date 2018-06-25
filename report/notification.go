package report

import "os/exec"

// ShowNotification show a notification desktop
func ShowNotification(title, summary string) {
	exec.Command("notify-send", title, summary).Run()
}
