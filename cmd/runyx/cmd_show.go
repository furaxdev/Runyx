package main

import "fmt"

func cmdShow(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: runyx show <model>")
	}
	_, store, err := loadStore()
	if err != nil {
		return err
	}
	mdl, ok, err := store.Get(args[0])
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("model %q not found; try `runyx list`", args[0])
	}
	fmt.Printf("Name:     %s\n", mdl.Name)
	fmt.Printf("File:     %s\n", mdl.File)
	fmt.Printf("Size:     %s\n", humanBytes(mdl.SizeBytes))
	fmt.Printf("Source:   %s\n", mdl.Source)
	fmt.Printf("SHA256:   %s\n", mdl.SHA256)
	fmt.Printf("Imported: %s\n", mdl.ImportedAt.Format("2006-01-02 15:04"))
	if mdl.Tuning.CtxSize > 0 {
		fmt.Printf("Tuning:   ctx=%d batch=%d threads=%d ngl=%d flash_attn=%v\n",
			mdl.Tuning.CtxSize, mdl.Tuning.BatchSize, mdl.Tuning.Threads, mdl.Tuning.NGPULayers, mdl.Tuning.FlashAttn)
	} else {
		fmt.Printf("Tuning:   not yet tuned (run `runyx tune %s` or `runyx run %s`)\n", mdl.Name, mdl.Name)
	}
	return nil
}
