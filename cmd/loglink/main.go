package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/exp/inotify"
)

func main() {
	log.SetLevel(log.InfoLevel)
	flag.Parse()

	if err := run(); err != nil {
		log.Fatalln(trace.DebugReport(err))
	}
}

func run() (err error) {
	if *targetDir == "" {
		return trace.BadParameter("target directory is required")
	}
	if len(watchDirs) == 0 {
		return trace.BadParameter("at least one watch directory is required")
	}
	*targetDir, err = filepath.Abs(*targetDir)
	if err != nil {
		return trace.Wrap(err, "failed to convert %v to absolute path", *targetDir)
	}
	if err = os.MkdirAll(*targetDir, sharedAccessMask); err != nil {
		return trace.Wrap(err, "failed to create directory `%v`", *targetDir)
	}
	log.Infof("symlinking logs in %v", *targetDir)
	log.Infof("watching %v", watchDirs)
	if err := createSymlinks(*targetDir, watchDirs); err != nil {
		return trace.Wrap(err, "failed to create symlinks")
	}

	watcher, err := createWatches(watchDirs)
	if err != nil {
		return trace.Wrap(err)
	}

	interrupt := make(chan os.Signal)
	signal.Notify(interrupt, os.Interrupt)

	var errWatch error
L:
	for {
		select {
		case ev := <-watcher.Event:
			log.Infof("inotify update for %v", ev)
			// Changed file - see if it requires symlinking
			if err = updateSymlinkIfNeeded(*targetDir, ev.Name); err != nil {
				log.Warningf("failed to symlink %v: %v", ev.Name, err)
			}
		case errWatch = <-watcher.Error:
			break L
		case <-interrupt:
			log.Infof("interrupted, closing")
			break L
		}
	}

	watcher.Close()
	return trace.Wrap(errWatch)
}

func createWatches(dirs []string) (*inotify.Watcher, error) {
	watcher, err := inotify.NewWatcher()
	if err != nil {
		return nil, trace.Wrap(err, "failed to create inotify watcher")
	}

	for _, dir := range dirs {
		err = watcher.AddWatch(dir, watchMask)
		if err != nil {
			watcher.Close()
			return nil, trace.Wrap(err, "failed to configure inotify watch for %v", dir)
		}
	}
	return watcher, nil
}

func createSymlinks(targetDir string, dirs []string) error {
	for _, dir := range dirs {
		if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return trace.Wrap(err)
			}
			if dir == path {
				return nil
			}
			if info.IsDir() {
				return filepath.SkipDir
			}
			return updateSymlinkIfNeeded(targetDir, path)
		}); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func updateSymlinkIfNeeded(targetDir, path string) (err error) {
	ext := filepath.Ext(path)
	if ext != ".log" {
		// Skip files that do not match the log file mask
		return nil
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return trace.Wrap(err)
	}
	symlinkFile := logSymlink(targetDir, path)
	_, err = os.Lstat(path)
	if os.IsNotExist(err) {
		log.Infof("original log file %v removed, will remove the symlink %v", path, symlinkFile)
		if err = os.Remove(symlinkFile); err != nil && os.IsNotExist(err) {
			err = nil
		}
		return trace.Wrap(err)
	}
	_, err = os.Lstat(symlinkFile)
	if os.IsNotExist(err) {
		log.Infof("new log file %v, will symlink as %v", path, symlinkFile)
		return trace.Wrap(os.Symlink(path, symlinkFile))
	}
	return nil
}

func logSymlink(targetDir, path string) string {
	podName := os.Getenv(EnvPodName)
	podNamespace := os.Getenv(EnvPodNamespace)
	podFullname := fmt.Sprintf("%v_%v", podName, podNamespace)
	containerName := os.Getenv(EnvContainerName)
	baseName := filepath.Base(path)
	return filepath.Join(targetDir, fmt.Sprintf("%v_%v-%v", podFullname, containerName, baseName))
}

var (
	targetDir = flag.String("target-dir", "", "target directory to create symlinks for all logs files in")
	watchDirs directories
)

func init() {
	flag.Var(&watchDirs, "watch-dir", "directory to watch for log files (*.log). Can be specified multiple times")
}

type directories []string

func (r directories) String() string {
	return fmt.Sprintf("directories(%v)", strings.Join(r, ","))
}

func (r *directories) Set(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return trace.Wrap(err)
	}
	if !info.IsDir() {
		return trace.BadParameter("%v is not a valid directory", dir)
	}
	*r = append(*r, dir)
	return nil
}

// watchMask defines a mask used to describe the events which inotify will send notifications for
const watchMask = inotify.IN_CREATE | inotify.IN_MOVED_TO | inotify.IN_MOVED_FROM | inotify.IN_DELETE

// sharedAccessMask defines the file access mask that the target directory is created with
// should it not exist prior to start
const sharedAccessMask = 0755

const (
	// EnvPodName names the pod
	EnvPodName = "POD_NAME"

	// EnvPodNamespace names the pod namespace
	EnvPodNamespace = "POD_NAMESPACE"

	// EnvContainerName names the container inside the pod
	EnvContainerName = "CONTAINER_NAME"
)
