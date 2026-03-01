package cmds

import (
	"arismcnc/database"
	"fmt"
	"io"

	"github.com/gliderlabs/ssh"
)

type CreditsCommand struct{}

func (c *CreditsCommand) Name() string {
	return "credits"
}

func (c *CreditsCommand) Execute(session ssh.Session, db *database.Database, args []string, output io.Writer) {
	fmt.Fprintln(output, "")
}

func (c *CreditsCommand) AdminOnly() bool {
	return false
}

func (c *CreditsCommand) Aliases() []string {
	return []string{"Credits", "credits"}
}

func init() {
	CommandMap["credits"] = &CreditsCommand{}
}
