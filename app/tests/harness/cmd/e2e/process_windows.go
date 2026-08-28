//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// Windows cannot signal process groups portably, so the child is placed in a
// Job Object with KILL_ON_JOB_CLOSE: terminating the job (or closing the last
// handle) reclaims the entire tree — cmd/npm → playwright → browsers.
//
// Conventions: Windows DLL calls return (r1, _, lastErr) where lastErr is
// always non-nil; success is judged by r1 == 0, never by lastErr == 0. If the
// child cannot be assigned to the job, it is killed immediately — a child
// running outside the job is exactly the orphan this file exists to prevent.
const (
	jobObjectExtendedLimitInformationClass = 9
	jobObjectLimitKillOnJobClose           = 0x00002000
	processTerminate                       = 0x0001
	processSetQuota                        = 0x0100
)

type jobBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type jobIOCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobExtendedLimitInformation struct {
	Basic                 jobBasicLimitInformation
	IoInfo                jobIOCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

var (
	kernel32                     = syscall.MustLoadDLL("kernel32.dll")
	procCreateJobObjectW         = kernel32.MustFindProc("CreateJobObjectW")
	procSetInformationJobObject  = kernel32.MustFindProc("SetInformationJobObject")
	procAssignProcessToJobObject = kernel32.MustFindProc("AssignProcessToJobObject")
	procOpenProcess              = kernel32.MustFindProc("OpenProcess")
	procTerminateJobObject       = kernel32.MustFindProc("TerminateJobObject")
)

// childJob holds the Job Object handle for the live child. Only one child is
// supervised at a time. pendingChildJob bridges job creation (before Start)
// to assignment (after Start produced a live PID).
var childJob syscall.Handle
var pendingChildJob syscall.Handle

// setChildProcessGroup creates the Job Object; assignment happens in
// startChild once Start has produced a live process handle.
func setChildProcessGroup(command *exec.Cmd) error {
	job, _, lastErr := procCreateJobObjectW.Call(0, 0)
	if job == 0 {
		return fmt.Errorf("create job object: %v", lastErr)
	}
	handle := syscall.Handle(job)

	limit := jobExtendedLimitInformation{}
	limit.Basic.LimitFlags = jobObjectLimitKillOnJobClose
	r1, _, lastErr := procSetInformationJobObject.Call(
		uintptr(handle),
		uintptr(jobObjectExtendedLimitInformationClass),
		uintptr(unsafe.Pointer(&limit)),
		uintptr(unsafe.Sizeof(limit)),
	)
	if r1 == 0 {
		syscall.CloseHandle(handle)
		return fmt.Errorf("configure job object: %v", lastErr)
	}

	pendingChildJob = handle
	return nil
}

// startChild starts the command and assigns it to the job. An assignment
// failure kills the child and returns an error so the supervisor never leaves
// a process running outside the job.
func startChild(command *exec.Cmd) error {
	if err := command.Start(); err != nil {
		if pendingChildJob != 0 {
			syscall.CloseHandle(pendingChildJob)
			pendingChildJob = 0
		}
		return err
	}
	if pendingChildJob != 0 && command.Process != nil {
		process, _, lastErr := procOpenProcess.Call(
			uintptr(processTerminate|processSetQuota),
			0,
			uintptr(command.Process.Pid),
		)
		if process == 0 {
			killErr := command.Process.Kill()
			closeErr := syscall.CloseHandle(pendingChildJob)
			pendingChildJob = 0
			if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				return fmt.Errorf("open child process for job assignment: %v; kill child: %w", lastErr, killErr)
			}
			if closeErr != nil {
				return fmt.Errorf("open child process for job assignment: %v; close job object: %w", lastErr, closeErr)
			}
			return fmt.Errorf("open child process for job assignment: %v", lastErr)
		}
		r1, _, lastErr := procAssignProcessToJobObject.Call(uintptr(pendingChildJob), process)
		syscall.CloseHandle(syscall.Handle(process))
		if r1 == 0 {
			killErr := command.Process.Kill()
			closeErr := syscall.CloseHandle(pendingChildJob)
			pendingChildJob = 0
			if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				return fmt.Errorf("assign child to job object: %v; kill child: %w", lastErr, killErr)
			}
			if closeErr != nil {
				return fmt.Errorf("assign child to job object: %v; close job object: %w", lastErr, closeErr)
			}
			return fmt.Errorf("assign child to job object: %v", lastErr)
		}
		childJob = pendingChildJob
	}
	pendingChildJob = 0
	return nil
}

func killProcessTree(command *exec.Cmd) error {
	if childJob != 0 {
		handle := childJob
		childJob = 0
		// Terminate the job explicitly (every process in it dies), then close
		// the handle as hygiene. If the explicit terminate fails, fall back
		// to the direct child, but still report the job failure: killing only
		// the direct child cannot prove that descendants were reclaimed.
		if r1, _, lastErr := procTerminateJobObject.Call(uintptr(handle), 1); r1 == 0 {
			var directErr error
			if command.Process != nil {
				directErr = command.Process.Kill()
				if directErr != nil && !errors.Is(directErr, os.ErrProcessDone) {
					directErr = fmt.Errorf("direct child kill: %w", directErr)
				}
			}
			_ = syscall.CloseHandle(handle)
			if directErr != nil {
				return fmt.Errorf("terminate job object: %v; %w", lastErr, directErr)
			}
			return fmt.Errorf("terminate job object: %v", lastErr)
		}
		return syscall.CloseHandle(handle)
	}
	if command.Process == nil {
		return nil
	}
	if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}
