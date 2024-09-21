package watcher

var globalExcludeDirs = []string{
	".git", ".svn", ".hg", // version control
	".idea", ".vscode", // IDEs
	".direnv",      // direnv nix guys
	"node_modules", // node
	".DS_Store",    // macOS
	".log",         // logs
}
