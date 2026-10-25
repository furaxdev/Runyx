package main

import "fmt"

func cmdList(args []string) error {
	_, store, err := loadStore()
	if err != nil {
		return err
	}
	models, err := store.List()
	if err != nil {
		return err
	}
	if len(models) == 0 {
		fmt.Println("No models installed yet. Try: runyx pull <owner/repo>")
		return nil
	}
	fmt.Printf("%-40s %-10s %-10s %s\n", "NAME", "SIZE", "SOURCE", "SHA256")
	for _, m := range models {
		sha := m.SHA256
		if len(sha) > 12 {
			sha = sha[:12]
		}
		fmt.Printf("%-40s %-10s %-10s %s\n", m.Name, humanBytes(m.SizeBytes), m.Source, sha)
	}
	return nil
}
