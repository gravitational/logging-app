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

	if *targetDir == "" {
		log.Fatalln("target directory is required")
	}
	if len(watchDirs) == 0 {
		log.Fatalln("at least one watch directory is required")
	}
	var err error
	*targetDir, err = filepath.Abs(*targetDir)
	if err != nil {
		log.Fatalln(err)
	}
	log.Infof("symlinking logs in %v", *targetDir)
	log.Infof("watching %v", watchDirs)
	if err := run(); err != nil {
		log.Fatalln(trace.DebugReport(err))
	}
}

func run() error {
	if err := createSymlinks(*targetDir, watchDirs); err != nil {
		return trace.Wrap(err, "failed to create symlinks")
	}

	watcher, err := inotify.NewWatcher()
	if err != nil {
		return trace.Wrap(err, "failed to create inotify watcher")
	}

	watchMask := inotify.IN_CREATE | inotify.IN_MOVED_TO | inotify.IN_MOVED_FROM | inotify.IN_DELETE
	for _, watchDir := range watchDirs {
		err = watcher.AddWatch(watchDir, watchMask)
		if err != nil {
			return trace.Wrap(err, "failed to create inotify watcher for %v", watchDir)
		}
	}

	interrupt := make(chan os.Signal)
	signal.Notify(interrupt, os.Interrupt)

	var errWatch error
L:
	for {
		select {
		case ev := <-watcher.Event:
			log.Infof("inotify update for %v", ev)
			// Changed file (new or deleted), see if it requires symlinking
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
			ext := filepath.Ext(path)
			if ext != ".log" {
				// Skip files that do not match the log file mask
				return nil
			}
			return updateSymlinkIfNeeded(targetDir, path)
		}); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func updateSymlinkIfNeeded(targetDir, path string) (err error) {
	path, err = filepath.Abs(path)
	if err != nil {
		return trace.Wrap(err)
	}
	symlinkFile := logSymlink(targetDir, path)
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return trace.Wrap(os.Remove(symlinkFile))
	}
	_, err = os.Lstat(symlinkFile)
	if os.IsNotExist(err) {
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

const (
	// EnvPodName names the pod
	EnvPodName = "POD_NAME"

	// EnvPodNamespace names the pod namespace
	EnvPodNamespace = "POD_NAMESPACE"

	// EnvContainerName names the container inside the pod
	EnvContainerName = "CONTAINER_NAME"
)
