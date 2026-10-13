package clock

import "time"

type System struct{}

// Now отдаёт UTC: часовой пояс — дело того, кто показывает время человеку.
func (System) Now() time.Time { return time.Now().UTC() }
