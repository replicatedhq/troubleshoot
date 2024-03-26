package util

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"strings"
)

func IsRunningAsRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		return false
	}

	// Check if the user ID is 0 (root's UID)
	return currentUser.Uid == "0"
}

func PromptYesNo(question string) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s (yes/no): ", question)

		response, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading response:", err)
			continue
		}

		response = strings.TrimSpace(response)
		response = strings.ToLower(response)

		if response == "yes" || response == "y" {
			return true
		} else if response == "no" || response == "n" {
			return false
		} else {
			fmt.Println("Please type 'yes' or 'no'.")
		}
	}
}
