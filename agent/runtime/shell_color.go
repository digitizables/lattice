package runtime

import (
	"os"
	"os/exec"
)

// Default LS_COLORS for environments (e.g. Debian slim) that ship without
// dircolors defaults — without this, GNU ls --color emits no escapes.
const defaultLSColors = "rs=0:di=01;34:ln=01;36:mh=00:pi=40;33:so=01;35:" +
	"do=01;35:bd=40;33;01:cd=40;33;01:or=40;31;01:mi=00:su=37;41:sg=30;43:" +
	"ca=00:tw=30;42:ow=34;42:st=37;44:ex=01;32:*.tar=01;31:*.tgz=01;31:" +
	"*.zip=01;31:*.gz=01;31:*.bz2=01;31:*.xz=01;31:*.7z=01;31:*.jpg=01;35:" +
	"*.jpeg=01;35:*.png=01;35:*.gif=01;35:*.webp=01;35:*.mp4=01;35:*.mkv=01;35"

// applyShellColorEnv makes common tools emit ANSI colors even when we capture
// stdout (ls, grep, git, etc.). Combined with a PTY on Unix, this matches a
// real interactive terminal.
func applyShellColorEnv(cmd *exec.Cmd) {
	env := os.Environ()
	env = append(env,
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"FORCE_COLOR=1",
		"CLICOLOR=1",
		"CLICOLOR_FORCE=1",
		"LS_COLORS="+defaultLSColors,
		// Reasonable width so columnar ls wraps within the console pane.
		"COLUMNS=100",
		"LINES=40",
	)
	cmd.Env = env
}
