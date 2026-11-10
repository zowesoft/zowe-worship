package settings

// Auto-projection keys. The switch is the operator's standing choice and is off
// until they turn it on; the pause is what F9 toggles, and the app writes it
// back to "true" at every start so nothing projects before someone is watching.
const (
	KeyAutoProjectionEnabled = "auto_projection_enabled"
	KeyAutoProjectionPaused  = "auto_projection_paused"
)
