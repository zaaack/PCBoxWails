//go:build darwin

package tray

import "fyne.io/systray"

type darwinTray struct{}

func (d *darwinTray) show() {}

func (d *darwinTray) remove() { systray.Quit() }

func (d *darwinTray) run(t *Tray) error {
	systray.Run(func() {
		if t.icon != nil {
			systray.SetIcon(t.icon)
		}
		systray.SetTooltip(t.tooltip)

		if t.menu != nil {
			for _, item := range t.menu.items {
				if item.separator {
					systray.AddSeparator()
				} else {
					mi := systray.AddMenuItem(item.label, item.label)
					go func(fn func()) {
						<-mi.ClickedCh
						fn()
					}(item.onClick)
				}
			}
		}

		if t.onDoubleClick != nil {
			systray.SetOnTapped(t.onDoubleClick)
		}
	}, func() {})
	return nil
}

func init() {
	newPlatformTray = func() platformTray { return &darwinTray{} }
}
