package main

import (
	"bufio"
	"fmt"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Define directories to clean
var cleanupPaths = []string{
	"/Users/chadooo/Library/Caches",
	"/Users/chadooo/Library/Logs",
}

// Loading animation (runs until task completes)
func startLoading(message string, wg *sync.WaitGroup) chan bool {
	stop := make(chan bool)
	go func() {
		frames := []string{"⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-stop:
				fmt.Printf("\r✅ %s\n", message)
				wg.Done()
				return
			default:
				fmt.Printf("\r%s %s", frames[i%len(frames)], message)
				i++
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
	return stop
}

// Get the total size of junk files in a directory
func getDirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil {
			size += info.Size()
		}
		return nil
	})
	return size
}

// Display junk file sizes
func showJunkUsage() {
	fmt.Println("\n🔍 Scanning junk files...")
	var totalSize int64
	for _, dir := range cleanupPaths {
		size := getDirSize(dir)
		totalSize += size
		fmt.Printf("📂 %s → %d MB\n", dir, size/1024/1024)
	}

	if totalSize == 0 {
		fmt.Println("\n✅ No junk files found! Your system is clean.")
		os.Exit(0)
	}

	fmt.Printf("\n🚨 Total Junk Size: %d MB 🚨\n", totalSize/1024/1024)
}

// Delete junk files
func cleanJunk() {
	fmt.Println("\n🗑️  Deleting junk files...")
	for _, dir := range cleanupPaths {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				_ = os.Remove(path)
			}
			return nil
		})
	}
	fmt.Println("✅ Junk files cleaned successfully!")
}

// Optimize memory
func optimizeMemory() {
	fmt.Println("\n🚀 Optimizing Memory...")

	if runtime.GOOS == "darwin" { // macOS
		cmd := exec.Command("sudo", "purge")
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			fmt.Println("⚠️  Memory optimization failed:", err)
			return
		}
	} else if runtime.GOOS == "linux" { // Linux
		cmd := exec.Command("sync")
		cmd.Run()
		cmd = exec.Command("sudo", "sysctl", "-w", "vm.drop_caches=3")
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			fmt.Println("⚠️  Memory optimization failed:", err)
			return
		}
	}

	fmt.Println("✅ Memory optimization complete!")
}

// Live system monitoring
func systemMonitor() {
	fmt.Println("\n📊 Live System Monitor (Press Ctrl+C to exit)")

	for {
		v, _ := mem.VirtualMemory()
		cpuPercent, _ := cpu.Percent(time.Second, false)

		fmt.Printf("\r🖥️ CPU Usage: %.2f%%  🏋️ RAM Usage: %.2f%%  (%.2f GB used of %.2f GB)  ",
			cpuPercent[0], v.UsedPercent, float64(v.Used)/1e9, float64(v.Total)/1e9)

		time.Sleep(2 * time.Second)
	}
}

// Disk usage scanner - Finds large files in a given directory
func scanLargeFiles(directory string, topN int) {
	fmt.Println("\n🔎 Scanning for large files in:", directory)

	var wg sync.WaitGroup
	wg.Add(1)
	stop := startLoading("Analyzing files...", &wg)

	files := []struct {
		Path string
		Size int64
	}{}

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files = append(files, struct {
				Path string
				Size int64
			}{Path: path, Size: info.Size()})
		}
		return nil
	})

	stop <- true
	wg.Wait()

	if err != nil {
		fmt.Println("⚠️  Error scanning directory:", err)
		return
	}

	// Sort by file size (largest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Size > files[j].Size
	})

	// Show top N largest files
	fmt.Printf("\n📂 Top %d largest files:\n", topN)
	for i, file := range files {
		if i >= topN {
			break
		}
		fmt.Printf("📄 %s → %.2f GB\n", file.Path, float64(file.Size)/1e9)
	}
}

// Ask for user confirmation
func promptUser(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\n⚠️  " + message + " (yes/no): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "yes" || input == "y"
}

// Main function
func main() {
	fmt.Println("🚀 CleanMyMac Go - Minimal Terminal Version 🚀")
	fmt.Println("===========================================")

	fmt.Printf("")

	showJunkUsage()

	// Prompt user before deleting
	if promptUser("Do you want to clean junk files?") {
		cleanJunk()
	} else {
		fmt.Println("❌ Cleanup canceled.")
	}

	// Scan for large files
	if promptUser("Do you want to scan for large files?") {
		fmt.Print("📂 Enter directory to scan: ")
		reader := bufio.NewReader(os.Stdin)
		dir, _ := reader.ReadString('\n')
		dir = strings.TrimSpace(dir)

		scanLargeFiles(dir, 5) // Show top 5 largest files
	}

	// Show system monitoring first
	go systemMonitor()

	// Optimize memory
	optimizeMemory()

	fmt.Println("\n⏳ Exiting in 3 seconds...")
	time.Sleep(3 * time.Second)
}
