package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"gopkg.in/yaml.v2"
)

// Config holds the application configuration
type Config struct {
	CleanupPaths []string `yaml:"cleanup_paths"`
	MaxFileSize  int64    `yaml:"max_file_size"` // in bytes
	TopFiles     int      `yaml:"top_files"`
	LogFile      string   `yaml:"log_file"`
}

// SystemCleaner handles the cleaning operations
type SystemCleaner struct {
	config     *Config
	logger     *log.Logger
	stopChan   chan struct{}
	operations *sync.WaitGroup
}

// FileInfo represents information about a file
type FileInfo struct {
	Path string
	Size int64
}

// NewSystemCleaner creates a new instance of SystemCleaner
func NewSystemCleaner(configPath string) (*SystemCleaner, error) {
	config, err := loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	logFile, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger := log.New(logFile, "", log.LstdFlags)

	return &SystemCleaner{
		config:     config,
		logger:     logger,
		stopChan:   make(chan struct{}),
		operations: &sync.WaitGroup{},
	}, nil
}

// loadConfig loads the configuration from a YAML file
func loadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	if err := yaml.Unmarshal(file, config); err != nil {
		return nil, err
	}

	return config, nil
}

// startLoading shows a loading animation
func (sc *SystemCleaner) startLoading(message string) chan bool {
	stop := make(chan bool)
	sc.operations.Add(1)

	go func() {
		frames := []string{"⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-stop:
				fmt.Printf("\r✅ %s\n", message)
				sc.operations.Done()
				return
			case <-sc.stopChan:
				fmt.Printf("\r❌ %s (interrupted)\n", message)
				sc.operations.Done()
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

// getDirSize calculates the total size of a directory
func (sc *SystemCleaner) getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// ShowJunkUsage displays information about junk files
func (sc *SystemCleaner) ShowJunkUsage() error {
	fmt.Println("\n🔍 Scanning junk files...")
	var totalSize int64

	fmt.Println("clean paths")
	fmt.Println(sc.config.CleanupPaths)

	for _, dir := range sc.config.CleanupPaths {
		size, err := sc.getDirSize(dir)
		if err != nil {
			sc.logger.Printf("Error scanning directory %s: %v", dir, err)
			continue
		}
		totalSize += size
		fmt.Printf("📂 %s → %d MB\n", dir, size/1024/1024)
	}

	if totalSize == 0 {
		fmt.Println("\n✅ No junk files found! Your system is clean.")
		return nil
	}

	fmt.Printf("\n🚨 Total Junk Size: %d MB 🚨\n", totalSize/1024/1024)
	return nil
}

// CleanJunk removes junk files
func (sc *SystemCleaner) CleanJunk() error {
	fmt.Println("\n🗑️  Deleting junk files...")

	fmt.Println("clean paths")
	fmt.Println(sc.config.CleanupPaths)

	for _, dir := range sc.config.CleanupPaths {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				sc.logger.Printf("Error accessing path %s: %v", path, err)
				return nil
			}
			if !info.IsDir() {
				if err := os.Remove(path); err != nil {
					sc.logger.Printf("Error removing file %s: %v", path, err)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("error cleaning directory %s: %w", dir, err)
		}
	}

	fmt.Println("✅ Junk files cleaned successfully!")
	return nil
}

// OptimizeMemory performs memory optimization based on the OS
func (sc *SystemCleaner) OptimizeMemory() error {
	fmt.Println("\n🚀 Optimizing Memory...")

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("sudo", "purge")
	case "linux":
		cmd = exec.Command("sudo", "sysctl", "-w", "vm.drop_caches=3")
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("memory optimization failed: %w", err)
	}

	fmt.Println("✅ Memory optimization complete!")
	return nil
}

// SystemMonitor provides real-time system monitoring
func (sc *SystemCleaner) SystemMonitor(ctx context.Context) {
	fmt.Println("\n📊 Live System Monitor (Press Ctrl+C to exit)")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			v, err := mem.VirtualMemory()
			if err != nil {
				sc.logger.Printf("Error getting memory info: %v", err)
				continue
			}

			cpuPercent, err := cpu.Percent(time.Second, false)
			if err != nil {
				sc.logger.Printf("Error getting CPU info: %v", err)
				continue
			}

			fmt.Printf("\r🖥️ CPU Usage: %.2f%%  🏋️ RAM Usage: %.2f%%  (%.2f GB used of %.2f GB)  ",
				cpuPercent[0], v.UsedPercent, float64(v.Used)/1e9, float64(v.Total)/1e9)
		}
	}
}

// ScanLargeFiles finds and reports large files in a directory
func (sc *SystemCleaner) ScanLargeFiles(directory string) error {
	fmt.Println("\n🔎 Scanning for large files in:", directory)

	stop := sc.startLoading("Analyzing files...")

	var files []FileInfo
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			sc.logger.Printf("Error accessing path %s: %v", path, err)
			return nil
		}
		if !info.IsDir() && info.Size() > sc.config.MaxFileSize {
			files = append(files, FileInfo{Path: path, Size: info.Size()})
		}
		return nil
	})

	stop <- true
	<-stop

	if err != nil {
		return fmt.Errorf("error scanning directory: %w", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Size > files[j].Size
	})

	fmt.Printf("\n📂 Top %d largest files:\n", sc.config.TopFiles)
	for i, file := range files {
		if i >= sc.config.TopFiles {
			break
		}
		fmt.Printf("📄 %s → %.2f GB\n", file.Path, float64(file.Size)/1e9)
	}

	return nil
}

// promptUser asks for user confirmation
func promptUser(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\n⚠️  " + message + " (yes/no): ")
	input, _ := reader.ReadString('\n')
	return strings.ToLower(strings.TrimSpace(input)) == "yes"
}

func main() {
	// Load configuration
	cleaner, err := NewSystemCleaner("config.yaml")
	if err != nil {
		log.Fatalf("Failed to initialize system cleaner: %v", err)
	}

	fmt.Println("🚀 System Cleaner Pro - v1.0.0 🚀")
	fmt.Println("=================================")

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n⚠️  Received interrupt signal. Cleaning up...")
		close(cleaner.stopChan)
		cancel()
	}()

	// Show junk usage
	if err := cleaner.ShowJunkUsage(); err != nil {
		cleaner.logger.Printf("Error showing junk usage: %v", err)
	}

	// Clean junk files if confirmed
	if promptUser("Do you want to clean junk files?") {
		if err := cleaner.CleanJunk(); err != nil {
			cleaner.logger.Printf("Error cleaning junk: %v", err)
		}
	}

	// Scan for large files if confirmed
	if promptUser("Do you want to scan for large files?") {
		fmt.Print("📂 Enter directory to scan: ")
		reader := bufio.NewReader(os.Stdin)
		dir, _ := reader.ReadString('\n')
		dir = strings.TrimSpace(dir)

		if err := cleaner.ScanLargeFiles(dir); err != nil {
			cleaner.logger.Printf("Error scanning large files: %v", err)
		}
	}

	// Start system monitoring
	go cleaner.SystemMonitor(ctx)

	// Optimize memory
	if err := cleaner.OptimizeMemory(); err != nil {
		cleaner.logger.Printf("Error optimizing memory: %v", err)
	}

	// Wait for all operations to complete
	cleaner.operations.Wait()

	fmt.Println("\n👋 Thank you for using System Cleaner Pro!")
}
