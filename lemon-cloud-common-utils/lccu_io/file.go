package lccu_io

import "os"

// 判断指定路径是否存在
func PathIsExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// 判断指定路径是否为文件夹
func PathIsDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

// 判断指定路径是否为文件
func PathIsFile(path string) bool {
	return !PathIsDir(path)
}

// 在指定路径创建文件夹，如果中间层文件夹不全，自动创建中间层文件夹
func PathMkdir(path string) error {
	return os.MkdirAll(path, os.ModePerm)
}

// 首先判断指定路径是否存在，如果不存在尝试创建文件夹，如果存在判断是否为文件夹，如果为文件那么删除，并重新创建文件夹，如果本身就是文件夹那么直接返回
func PathDirRepair(path string) error {
	if exists, err := PathIsExists(path); err != nil {
		// 检测是否存在时出错
		return err
	} else if !exists {
		// 文件夹不存在，创建
		if err = PathMkdir(path); err != nil {
			// 创建文件夹出错
			return err
		}
	} else {
		if !PathIsDir(path) {
			// 存在，但不是文件夹，删除文件并创建文件夹
			if err = os.Remove(path); err != nil {
				// 删除路径处文件失败
				return err
			}
			if err = PathMkdir(path); err != nil {
				return err
			}
		}
	}
	return nil
}
