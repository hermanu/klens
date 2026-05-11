package components

import "strings"

// Command is one entry in the shell's command vocabulary. Used by both the
// modal palette (ctrl+p) and the inline ex-mode prompt (`:`), so the two
// surfaces share a single source of truth for what's runnable.
//
// Name is the canonical spelling typed in the modal palette; Alias is the
// short form (vim-style `:dp`). Either matches in FilterCommands.
type Command struct {
	Name  string // e.g. "deployments"
	Desc  string // e.g. "list deployments"
	Alias string // e.g. ":dp"
}

// DefaultCommands is the built-in command list — resource jumps, the runtime
// context switcher, and quit. Adding a new command here makes it reachable
// from both ctrl+p (modal) and `:` (inline) automatically.
func DefaultCommands() []Command {
	return []Command{
		{Name: "pods", Desc: "list pods", Alias: ":po"},
		{Name: "deployments", Desc: "list deployments", Alias: ":dp"},
		{Name: "services", Desc: "list services", Alias: ":svc"},
		{Name: "secrets", Desc: "list secrets", Alias: ":sec"},
		{Name: "configmaps", Desc: "list configmaps", Alias: ":cm"},
		{Name: "namespaces", Desc: "list namespaces", Alias: ":ns"},
		{Name: "nodes", Desc: "list nodes", Alias: ":no"},
		{Name: "pvcs", Desc: "list persistent volume claims", Alias: ":pvc"},
		{Name: "all", Desc: "show all namespaces (clear scope)", Alias: ":all"},
		{Name: "context", Desc: "switch cluster", Alias: ":ctx"},
		{Name: "quit", Desc: "exit klens", Alias: ":q"},
	}
}

// FilterCommands returns commands matching q (case-insensitive substring on
// Name or Alias). Empty q returns the full slice in declaration order so the
// modal palette has a stable rendering when first opened.
//
// Alias matching is prefix-anchored *after* the leading `:` is stripped — so
// `dp` finds `:dp` but `p` doesn't match every command whose alias starts
// with `:p…`. Substring on Name still applies, so typing `de` still surfaces
// `deployments`.
func FilterCommands(cmds []Command, q string) []Command {
	q = strings.ToLower(strings.TrimSpace(q))
	q = strings.TrimPrefix(q, ":")
	if q == "" {
		return cmds
	}
	var out []Command
	for _, c := range cmds {
		alias := strings.TrimPrefix(c.Alias, ":")
		if strings.Contains(strings.ToLower(c.Name), q) || strings.HasPrefix(strings.ToLower(alias), q) {
			out = append(out, c)
		}
	}
	return out
}

// ExactCommand returns the command whose Name or Alias matches q exactly,
// or nil. Used by the inline ex-mode to short-circuit Enter when the user
// typed an unambiguous key like `q` or `dp`, even if the substring match
// would have surfaced multiple candidates.
func ExactCommand(cmds []Command, q string) *Command {
	q = strings.ToLower(strings.TrimSpace(q))
	q = strings.TrimPrefix(q, ":")
	if q == "" {
		return nil
	}
	for i := range cmds {
		alias := strings.TrimPrefix(cmds[i].Alias, ":")
		if strings.EqualFold(cmds[i].Name, q) || strings.EqualFold(alias, q) {
			return &cmds[i]
		}
	}
	return nil
}

// LongestCommonPrefix returns the longest case-insensitive prefix shared by
// every command's Name in cmds. Used by Tab-complete in inline ex-mode: if
// the user types `dep` and only `deployments` matches, Tab fills in the rest.
// If multiple candidates match, Tab fills in only the shared prefix so the
// user sees the disambiguation point.
func LongestCommonPrefix(cmds []Command) string {
	if len(cmds) == 0 {
		return ""
	}
	prefix := strings.ToLower(cmds[0].Name)
	for _, c := range cmds[1:] {
		name := strings.ToLower(c.Name)
		i := 0
		for i < len(prefix) && i < len(name) && prefix[i] == name[i] {
			i++
		}
		prefix = prefix[:i]
		if prefix == "" {
			return ""
		}
	}
	return prefix
}
