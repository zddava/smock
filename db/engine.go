package db

import (
	"log"
	"os"

	"github.com/fsnotify/fsnotify"
)

type Engine struct {
	Watcher *fsnotify.Watcher
}

func Create(folder string) (*Engine, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	err = watcher.Add(folder)
	if err != nil {
		return nil, err
	}

	i := 0
	go func() {
		for {
			select {
			case ev := <-watcher.Events:
				{
					i++
					if ev.Op.Has(fsnotify.Create) {
						log.Println("创建文件 : ", ev.Name, "   -", i)
					}
					if ev.Op&fsnotify.Write == fsnotify.Write {
						log.Println("写入文件 : ", ev.Name, "   -", i)
						bytes, _ := os.ReadFile(ev.Name)
						log.Println(bytes)
					}
					if ev.Op&fsnotify.Remove == fsnotify.Remove {
						log.Println("删除文件 : ", ev.Name, "   -", i)
					}
					if ev.Op&fsnotify.Rename == fsnotify.Rename {
						log.Println("重命名文件 : ", ev.Name, "   -", i)
						_, err := os.Stat(ev.Name)
						log.Println(os.IsExist(err))
					}
					if ev.Op&fsnotify.Chmod == fsnotify.Chmod {
						log.Println("修改权限 : ", ev.Name, "   -", i)
					}
				}
			case err := <-watcher.Errors:
				{
					log.Println("error : ", err)
					return
				}
			}
		}
	}()

	return &Engine{Watcher: watcher}, nil
}

func (engine *Engine) Close(folder string) (err error) {
	return engine.Watcher.Close()
}
