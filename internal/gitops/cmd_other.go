//go:build !windows

package gitops

import "syscall"

func getSysProcAttr() *syscall.SysProcAttr {
	return nil
}
