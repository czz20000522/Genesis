package main

import (
	"context"
	"strings"
)

const (
	serviceKindKernel = "kernel"

	serviceOwnershipOwned    = "owned"
	serviceOwnershipExternal = "external"

	sidecarStartNotImplemented      = "sidecar_start_not_implemented"
	sidecarExternalKernelConfigured = "external_kernel_configured"
	sidecarStopped                  = "sidecar_stopped"
)

type LocalServiceSupervisorConfig struct {
	KernelBaseURL string
	RuntimeToken  string
	External      bool
}

type LocalServiceSupervisor struct {
	cfg            LocalServiceSupervisorConfig
	status         SidecarStatus
	startAttempted bool
	stopAttempted  bool
}

func NewLocalServiceSupervisor(cfg LocalServiceSupervisorConfig) *LocalServiceSupervisor {
	cfg.KernelBaseURL = strings.TrimRight(strings.TrimSpace(cfg.KernelBaseURL), "/")
	cfg.RuntimeToken = strings.TrimSpace(cfg.RuntimeToken)
	supervisor := &LocalServiceSupervisor{cfg: cfg}
	supervisor.status = supervisor.initialKernelStatus()
	return supervisor
}

func (s *LocalServiceSupervisor) KernelStatus() SidecarStatus {
	if s == nil {
		return SidecarStatus{}
	}
	return s.status
}

func (s *LocalServiceSupervisor) StartKernel(context.Context) SidecarStatus {
	if s == nil {
		return SidecarStatus{}
	}
	if s.cfg.External {
		s.status = s.initialKernelStatus()
		return s.status
	}
	s.startAttempted = true
	s.status = ownedKernelStatus(sidecarStartNotImplemented)
	return s.status
}

func (s *LocalServiceSupervisor) StopOwned(context.Context) SidecarStatus {
	if s == nil {
		return SidecarStatus{}
	}
	if s.cfg.External {
		s.status = s.initialKernelStatus()
		return s.status
	}
	s.stopAttempted = true
	s.status = ownedKernelStatus(sidecarStopped)
	return s.status
}

func (s *LocalServiceSupervisor) initialKernelStatus() SidecarStatus {
	if s.cfg.External {
		return SidecarStatus{
			ServiceID: "kernel",
			Kind:      serviceKindKernel,
			Ownership: serviceOwnershipExternal,
			Readiness: "not_ready",
			Reason:    sidecarExternalKernelConfigured,
		}
	}
	return ownedKernelStatus(sidecarStartNotImplemented)
}

func ownedKernelStatus(reason string) SidecarStatus {
	return SidecarStatus{
		ServiceID: "kernel",
		Kind:      serviceKindKernel,
		Ownership: serviceOwnershipOwned,
		Readiness: "not_ready",
		Reason:    reason,
	}
}
