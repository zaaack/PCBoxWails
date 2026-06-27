//go:build windows

package tray

import "github.com/gogpu/systray"

type windowsTray struct {
	inner *systray.SystemTray
}

func (w *windowsTray) show() { w.inner.Show() }

func (w *windowsTray) remove() { w.inner.Remove() }

func (w *windowsTray) run(t *Tray) error {
	if t.icon != nil {
		w.inner.SetIcon(t.icon)
	}
	w.inner.SetTooltip(t.tooltip)

	if t.menu != nil {
		menu := systray.NewMenu()
		for _, item := range t.menu.items {
			if item.separator {
				menu.AddSeparator()
			} else {
				menu.Add(item.label, item.onClick)
			}
		}
		w.inner.SetMenu(menu)
	}

	if t.onClick != nil {
		w.inner.OnClick(t.onClick)
	}

	if t.onDoubleClick != nil {
		w.inner.OnDoubleClick(t.onDoubleClick)
	}

	w.inner.Show()
	return w.inner.Run()
}

func init() {
	newPlatformTray = func() platformTray {
		return &windowsTray{inner: systray.New()}
	}
}
