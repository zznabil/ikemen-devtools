package mutation

// ToolNames returns mutation operations only when write capability is explicit.
func ToolNames(writeEnabled bool) []string {
	if !writeEnabled {
		return nil
	}
	return []string{"rename.prepare", "fix.prepare", "patch.plan", "patch.diff", "patch.apply", "patch.recover"}
}
