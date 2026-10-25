package main

import (
	"fmt"

	"github.com/furaxdev/runyx/internal/hwdetect"
	"github.com/furaxdev/runyx/internal/modelstore"
)

func cmdTune(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: runyx tune <model>")
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

	info := hwdetect.Detect()
	t := hwdetect.Tune(info, int(mdl.SizeBytes/(1024*1024)))
	mdl.Tuning = modelstore.TuningParams{
		NGPULayers: t.NGPULayers,
		Threads:    t.Threads,
		CtxSize:    t.CtxSize,
		BatchSize:  t.BatchSize,
		FlashAttn:  t.FlashAttn,
	}
	if err := store.Put(mdl); err != nil {
		return err
	}
	fmt.Printf("Tuned %q: ctx=%d batch=%d threads=%d ngl=%d flash_attn=%v\n",
		mdl.Name, t.CtxSize, t.BatchSize, t.Threads, t.NGPULayers, t.FlashAttn)
	return nil
}
