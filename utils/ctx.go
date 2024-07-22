// Package utils provides utility functions for the Keploy application.
package utils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

var cancel context.CancelFunc

func NewCtx() context.Context {
	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())

	SetCancel(cancel)
	// Set up a channel to listen for signals
	sigs := make(chan os.Signal, 1)
	// os.Interrupt is more portable than syscall.SIGINT
	// there is no equivalent for syscall.SIGTERM in os.Signal
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	// Start a goroutine that will cancel the context when a signal is received
	go func() {
		<-sigs
		fmt.Println("Signal received, canceling context...")
		cancel()
	}()

	return ctx
}
// ReadKeployConfig reads the .keploy file and returns its contents as a map.
func ReadKeployConfig() (map[string]string, error) {
	config := make(map[string]string)
	file, err := os.Open(filepath.Join(os.Getenv("HOME"), ".keploy"))
	if err != nil {
		return config, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "=")
		if len(parts) == 2 {
			config[parts[0]] = parts[1]
		}
	}

	return config, scanner.Err()
}

// WriteKeployConfig writes the given config map to the .keploy file.
func WriteKeployConfig(config map[string]string) error {
	file, err := os.Create(filepath.Join(os.Getenv("HOME"), ".keploy"))
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for key, value := range config {
		fmt.Fprintf(writer, "%s=%s\n", key, value)
	}

	return writer.Flush()
}

func getLatestRelease() (string, error) {
	url := "https://api.github.com/repos/keploy/keploy/releases/latest"
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch latest release: %v", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var release Release
	err = json.Unmarshal(body, &release)
	if err != nil {
		return "", err
	}

	return release.TagName, nil
}

func promptUpdate(currentVersion, latestVersion string) bool {
	fmt.Printf("A new version of Keploy is available: %s (current version: %s)\n", latestVersion, currentVersion)
	fmt.Print("Do you want to update to the latest version? [Y/n]: ")

	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	return response == "y" || response == "yes" || response == ""
}

func savePreference(updatePref string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	keployFile := filepath.Join(homeDir, ".keploy")
	lines, err := ioutil.ReadFile(keployFile)
	if err != nil {
		return err
	}

	content := strings.Split(string(lines), "\n")
	for i, line := range content {
		if strings.HasPrefix(line, "update_pref=") {
			content[i] = "update_pref=" + updatePref
			break
		}
	}

	return ioutil.WriteFile(keployFile, []byte(strings.Join(content, "\n")), 0644)
}

func checkUpdatePreference() (bool, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}

	keployFile := filepath.Join(homeDir, ".keploy")
	lines, err := ioutil.ReadFile(keployFile)
	if err != nil {
		return false, err
	}

	for _, line := range strings.Split(string(lines), "\n") {
		if strings.HasPrefix(line, "update_pref=") {
			return strings.TrimSpace(strings.Split(line, "=")[1]) == "no", nil
		}
	}

	// Default preference if not found
	return false, nil
}

func logWarning(latestVersion string) {
	fmt.Printf("Warning: A new version of Keploy is available: %s.\n", latestVersion)
	fmt.Println("To update, run: keploy update")
}

func checkForUpdates() {
	currentVersion := version // Using the version variable defined globally
	latestVersion, err := getLatestRelease()
	if err != nil {
		fmt.Printf("Error checking for latest release: %v\n", err)
		return
	}

	if latestVersion != "" && currentVersion != latestVersion {
		updatePref, err := checkUpdatePreference()
		if err != nil {
			fmt.Printf("Error checking update preference: %v\n", err)
			return
		}

		if !updatePref {
			if promptUpdate(currentVersion, latestVersion) {
				// Code to update Keploy
			} else {
				if err := savePreference("no"); err != nil {
					fmt.Printf("Error saving update preference: %v\n", err)
					return
				}
				logWarning(latestVersion)
			}
		} else {
			logWarning(latestVersion)
		}
	}
}
// Stop requires a reason to stop the server.
// this is to ensure that the server is not stopped accidentally.
// and to trace back the stopper
func Stop(logger *zap.Logger, reason string) error {
	// Stop the server.
	if logger == nil {
		return errors.New("logger is not set")
	}
	if cancel == nil {
		err := errors.New("cancel function is not set")
		LogError(logger, err, "failed stopping keploy")
		return err
	}

	if reason == "" {
		err := errors.New("cannot stop keploy without a reason")
		LogError(logger, err, "failed stopping keploy")
		return err
	}

	logger.Info("stopping Keploy", zap.String("reason", reason))
	ExecCancel()
	return nil
}

func ExecCancel() {
	cancel()
}

func SetCancel(c context.CancelFunc) {
	cancel = c
}
