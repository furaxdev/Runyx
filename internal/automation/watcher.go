package automation

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
)

// FolderWatcher watches a directory and invokes OnFile for each new or
// modified file ("every morning, summarize this folder" style
// automation — the actual summarization action is the caller's
// business).
type FolderWatcher struct {
	watcher *fsnotify.Watcher
	OnFile  func(path string, op string)
	OnError func(error)
	done    chan struct{}
}

// WatchFolder starts watching dir (non-recursively; add subdirectories
// explicitly if needed) and returns a FolderWatcher. Call Close to stop.
func WatchFolder(dir string, onFile func(path, op string)) (*FolderWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}
	if err := w.Add(dir); err != nil {
		w.Close()
		return nil, fmt.Errorf("watch %s: %w", dir, err)
	}

	fw := &FolderWatcher{watcher: w, OnFile: onFile, done: make(chan struct{})}
	go fw.loop()
	return fw, nil
}

func (fw *FolderWatcher) loop() {
	defer close(fw.done)
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Create|fsnotify.Write) != 0 && fw.OnFile != nil {
				fw.OnFile(event.Name, event.Op.String())
			}
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			if fw.OnError != nil {
				fw.OnError(err)
			}
		}
	}
}

// Close stops watching.
func (fw *FolderWatcher) Close() error {
	err := fw.watcher.Close()
	<-fw.done
	return err
}
