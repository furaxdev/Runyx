package main

import (
	"fmt"

	"github.com/furaxdev/runyx/internal/hwdetect"
)

func cmdDoctor(args []string) error {
	info := hwdetect.Detect()

	fmt.Println("Runyx doctor — hardware report")
	fmt.Println()
	fmt.Printf("OS/Arch:      %s/%s\n", info.OS, info.Arch)
	if info.CPUModel != "" {
		fmt.Printf("CPU:          %s (%d threads)\n", info.CPUModel, info.CPUThreads)
	} else {
		fmt.Printf("CPU:          %d threads\n", info.CPUThreads)
	}
	fmt.Printf("AVX2:         %v\n", info.HasAVX2)
	fmt.Printf("AVX512:       %v\n", info.HasAVX512)
	if info.TotalRAMMB > 0 {
		fmt.Printf("RAM:          %.1f GB\n", float64(info.TotalRAMMB)/1024)
	} else {
		fmt.Printf("RAM:          could not detect\n")
	}
	if len(info.GPUs) == 0 {
		fmt.Println("GPU:          none detected (CPU-only inference)")
	} else {
		for _, g := range info.GPUs {
			if g.VRAMMB > 0 {
				fmt.Printf("GPU:          %s (%s backend, %.1f GB VRAM)\n", g.Name, g.Backend, float64(g.VRAMMB)/1024)
			} else {
				fmt.Printf("GPU:          %s (%s backend, VRAM unknown)\n", g.Name, g.Backend)
			}
		}
	}
	fmt.Println()
	fmt.Printf("Recommended backend: %s\n", info.BestBackend())
	return nil
}
