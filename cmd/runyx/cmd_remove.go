package main

import "fmt"

func cmdRemove(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: runyx rm <model>")
	}
	_, store, err := loadStore()
	if err != nil {
		return err
	}
	if err := store.Remove(args[0], true); err != nil {
		return err
	}
	fmt.Printf("Removed %q\n", args[0])
	return nil
}
