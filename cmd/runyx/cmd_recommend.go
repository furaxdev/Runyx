package main

import (
	"fmt"

	"github.com/furaxdev/runyx/internal/hwdetect"
)

func cmdRecommend(args []string) error {
	info := hwdetect.Detect()
	recs := hwdetect.Recommend(info)

	if len(recs) == 0 {
		fmt.Println("No model in the default catalog fits comfortably on this machine.")
		return nil
	}

	fmt.Printf("Recommended models for this machine (backend: %s):\n\n", info.BestBackend())
	for _, r := range recs {
		fmt.Printf("  %-28s %-10s ~%5.1f GB  %s  (runyx pull %s)\n",
			r.Name, r.Quant, float64(r.EstSizeMB)/1024, r.Headroom, r.Repo)
	}
	return nil
}
