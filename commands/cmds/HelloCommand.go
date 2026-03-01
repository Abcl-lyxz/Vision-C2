package cmds

import (
	"arismcnc/database"
	"fmt"
	"io"

	"github.com/gliderlabs/ssh"
)

// HelloCommand example command, not restricted to admins
type HelloCommand struct{}

func (c *HelloCommand) Name() string {
	return "hello"
}

func (c *HelloCommand) Execute(session ssh.Session, db *database.Database, args []string, output io.Writer) {
	fmt.Fprintln(output, "Hello from the HelloCommand!")
}

func (c *HelloCommand) AdminOnly() bool {
	return false
}

// Aliases for HelloCommand
func (c *HelloCommand) Aliases() []string {
	return []string{"hi"}
}

// Register HelloCommand in the CommandMap
func init() {
	CommandMap["hello"] = &HelloCommand{}
}
