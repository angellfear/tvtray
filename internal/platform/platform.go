// Package platform hides OS-specific power and display notifications.
// Porting to another OS means implementing WatchPower for it.
package platform

// PowerEvent is a display or system power transition.
type PowerEvent int

const (
	DisplayOff PowerEvent = iota
	DisplayOn
	Suspend // the system is about to sleep; handler runs synchronously
	Resume
)

func (e PowerEvent) String() string {
	return [...]string{"display-off", "display-on", "suspend", "resume"}[e]
}

// PowerHandler is called for each event. For Suspend the OS waits for the
// handler to return (Windows allows about 2 seconds), so it should be quick.
type PowerHandler func(PowerEvent)
