package main

import (
	"github.com/fsnotify/fsnotify"
	"github.com/bestmethod/go-logger"
	"fmt"
	"os"
	"time"
	"os/exec"
	"strings"
	"syscall"
	"github.com/deckarep/gosx-notifier"
	"github.com/everdev/mack"
)

var sourceDir = "testSource/" // will be passed to rsync as source, preferably add trailing / for sanity

var destinationDir = "testDestination/" // will be passed to rsync as destination, add trailing / for sanity

var volumeName = "External" // the name of the volume, as it appears inside the Volume directory

var rsyncDelete = false // should we pass --delete to rsync, depends if you want to sync deletes

var rsyncSwitches = "-av" // do only -a if you don't want verbose. Since we are local, no need for compression in transit. n == dry run

var rsyncPath = "rsync" // on osx it should be in path, so no need for a full path, but who knows

var fsNotifyWatch = "/private/tmp" // matches plist stuff

var fsNotifyFile = "volumes-changed" // matches plist stuff

func main() {
	// setup logger
	logger := new(Logger.Logger)
	err := logger.Init("", "rsync-on-mount", Logger.LEVEL_DEBUG | Logger.LEVEL_INFO | Logger.LEVEL_WARN, Logger.LEVEL_ERROR | Logger.LEVEL_CRITICAL, Logger.LEVEL_NONE)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CRITICAL Could not initialize logger. Quitting. Details: %s\n", err)
		os.Exit(1)
	}

	logger.Info(fmt.Sprintf("Source: %s, Destination: %s, Volume Name: %s, rsyncDelete: %t, rsyncSwitches: %s, fsNotifyPath: %s/%s",sourceDir,destinationDir,volumeName,rsyncDelete,rsyncSwitches,fsNotifyWatch,fsNotifyFile))

	// setup fsnotify
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Fatal(fmt.Sprintf("Could not create fsnotify watcher: %s",err),2)
	}

	// make channel
	done := make(chan bool)

	// Process events
	go func() {
		var cmd *exec.Cmd
		var volMounted, volMountedCheck bool
		var err error
		volMounted, err = getVolState()
		if err != nil { logger.Error(fmt.Sprintf("Could not get volume state in /Volumes: %s",err)) }
		logger.Info(fmt.Sprintf("Starting goroutine, volume mounted state: %t",volMounted))
		for {
			select {
			case ev := <-watcher.Events:
				logger.Debug(fmt.Sprintf("Event: %s",ev))
				if ev.Name == fmt.Sprintf("%s/%s",fsNotifyWatch,fsNotifyFile) {
					volMountedCheck, err = getVolState()
					if err != nil {
						logger.Error(fmt.Sprintf("Could not get volume state in /Volumes: %s", err))
					}
					if volMounted != volMountedCheck {
						volMounted = volMountedCheck
						if volMounted == true {
							// volume has just been mounted
							logger.Info("CREATE event, waiting for mount")
							// wait for mount to happen BEFORE rsync runs
							var mounted= 5 // we check once per second, until mounted is 0. Once 0, we Error.
							for {
								out, err := exec.Command("mount").CombinedOutput()
								if err != nil {
									logger.Fatal(fmt.Sprintf("Failed 'mount' command, this must be fatal: %s", err), 4)
								}
								if strings.Contains(string(out), fmt.Sprintf("/Volumes/%s", volumeName)) {
									break;
								} else {
									mounted = mounted - 1
									if mounted == 0 {
										break;
									} else {
										time.Sleep(1 * time.Second)
									}
								}
							}
							if mounted == 0 {
								logger.Error("Tried to find the directory from fsnotify in mount for 5 seconds, but failed. Will not run rsync.")
							} else {
								logger.Info("Running rsync...")
								// rsync here
								if rsyncDelete == true {
									cmd = exec.Command(rsyncPath, rsyncSwitches, "--delete", sourceDir, destinationDir)
								} else {
									cmd = exec.Command(rsyncPath, rsyncSwitches, sourceDir, destinationDir)
								}
								// the below goroutine will run the command with .CombinedOutput to get stdout and stderr of rsync as well.
								// it's in a separate goroutine since it is a blocking call
								go MonitorRsync(logger, cmd)
							}
						} else {
							// volume has just been unmounted
							logger.Info("REMOVE event")
							// rsync kill here, IF not running
							if cmd != nil {
								processState := cmd.Process.Signal(syscall.Signal(0))
								if processState == nil {
									logger.Warn("Have to kill rsync, doing it")
									err := cmd.Process.Kill()
									if err != nil {
										logger.Error(fmt.Sprintf("Cannot kill rsync when we should be able to: %s", err)) // error as can't kill a process.
									}
								}
							}
						}
					} else {
						logger.Info("State in /Volumes changed but nothing to do with us")
					}
				}
			case err := <-watcher.Errors:
				logger.Error(fmt.Sprintf("Fsnotify watcher error: %s",err))
			}
		}
	}()

	// add watcher
	err = watcher.Add(fsNotifyWatch)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Could not add directory watcher for fsnotify: %s",err),3)
	}

	logger.Info("Added fsnotify watcher, waiting...")
	// Hang so program doesn't exit
	<-done

	// cleanup, we could kill rsync here if this exits and rsync is running... but why would you.
	watcher.Close()
}

func MonitorRsync(logger *Logger.Logger, cmd *exec.Cmd) {
	note := gosxnotifier.NewNotification("Starting rSync...")
	note.Push()
	rsyncOutput, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error(fmt.Sprintf("Could not start rsync or rsync finished non-zero: %s",err))
		go mack.Alert("rSync ERROR",fmt.Sprintf("rSync exit with error: %s",err),"critical")
	} else {
		go mack.Alert("rSync Completed successfully","rSync Completed successfully")
	}
	logger.Debug(fmt.Sprintf("RSYNC OUTPUT FOLLOWS:\n%s",rsyncOutput))
	return
}

func getVolState() (bool,error) {
	_, err := os.Stat(fmt.Sprintf("/Volumes/%s",volumeName))
	if err == nil { return true, nil }
	if os.IsNotExist(err) { return false, nil }
	return true, err
}