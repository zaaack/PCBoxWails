package tray

var newPlatformTray func() platformTray

type platformTray interface {
	show()
	remove()
	run(t *Tray) error
}

type Tray struct {
	icon          []byte
	tooltip       string
	menu          *Menu
	onDoubleClick func()
	impl          platformTray
}

type Menu struct {
	items []menuItem
}

type menuItem struct {
	label     string
	onClick   func()
	separator bool
}

func New() *Tray {
	return &Tray{}
}

func (t *Tray) SetIcon(data []byte) {
	t.icon = data
}

func (t *Tray) SetTooltip(text string) {
	t.tooltip = text
}

func (t *Tray) SetMenu(menu *Menu) {
	t.menu = menu
}

func (t *Tray) OnDoubleClick(fn func()) {
	t.onDoubleClick = fn
}

func (t *Tray) Show() {
	t.impl.show()
}

func (t *Tray) Remove() {
	t.impl.remove()
}

func (t *Tray) Run() error {
	t.impl = newPlatformTray()
	return t.impl.run(t)
}

func NewMenu() *Menu {
	return &Menu{}
}

func (m *Menu) Add(label string, onClick func()) {
	m.items = append(m.items, menuItem{label: label, onClick: onClick})
}

func (m *Menu) AddSeparator() {
	m.items = append(m.items, menuItem{separator: true})
}
