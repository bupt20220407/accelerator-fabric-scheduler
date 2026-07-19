package deviceplugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/validation"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type Config struct {
	ResourceName   string
	Count          int
	DevicesPerNUMA int
	SocketDir      string
}

type Plugin struct {
	pluginapi.UnimplementedDevicePluginServer

	config     Config
	endpoint   string
	socketPath string

	mu      sync.RWMutex
	devices map[string]*pluginapi.Device
	updates chan struct{}
}

func New(config Config) (*Plugin, error) {
	if errs := validation.IsQualifiedName(config.ResourceName); len(errs) > 0 || !strings.Contains(config.ResourceName, "/") {
		return nil, fmt.Errorf("resourceName %q is not a qualified extended resource name", config.ResourceName)
	}
	if config.Count < 1 {
		return nil, fmt.Errorf("count must be positive")
	}
	if config.DevicesPerNUMA < 1 {
		return nil, fmt.Errorf("devicesPerNUMA must be positive")
	}
	if config.SocketDir == "" {
		config.SocketDir = pluginapi.DevicePluginPath
	}

	endpoint := socketName(config.ResourceName)
	plugin := &Plugin{
		config:     config,
		endpoint:   endpoint,
		socketPath: filepath.Join(config.SocketDir, endpoint),
		devices:    make(map[string]*pluginapi.Device, config.Count),
		updates:    make(chan struct{}, 1),
	}
	idPrefix := strings.NewReplacer("/", "-", ".", "-", "_", "-").Replace(strings.ToLower(config.ResourceName))
	for index := 0; index < config.Count; index++ {
		id := fmt.Sprintf("%s-%d", idPrefix, index)
		plugin.devices[id] = &pluginapi.Device{
			ID:     id,
			Health: pluginapi.Healthy,
			Topology: &pluginapi.TopologyInfo{Nodes: []*pluginapi.NUMANode{{
				ID: int64(index / config.DevicesPerNUMA),
			}}},
		}
	}
	return plugin, nil
}

func (p *Plugin) Run(ctx context.Context) error {
	if err := os.MkdirAll(p.config.SocketDir, 0o755); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}
	if err := os.Remove(p.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	listener, err := net.Listen("unix", p.socketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", p.socketPath, err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(p.socketPath)
	}()

	server := grpc.NewServer()
	pluginapi.RegisterDevicePluginServer(server, p)
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.Serve(listener)
	}()

	if err := p.register(ctx); err != nil {
		server.Stop()
		return err
	}

	select {
	case <-ctx.Done():
		server.GracefulStop()
		return nil
	case err := <-serveErrors:
		if err == nil {
			return nil
		}
		return fmt.Errorf("serve device plugin: %w", err)
	}
}

func (p *Plugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{GetPreferredAllocationAvailable: true}, nil
}

func (p *Plugin) ListAndWatch(_ *pluginapi.Empty, stream grpc.ServerStreamingServer[pluginapi.ListAndWatchResponse]) error {
	if err := stream.Send(&pluginapi.ListAndWatchResponse{Devices: p.deviceList()}); err != nil {
		return err
	}
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-p.updates:
			if err := stream.Send(&pluginapi.ListAndWatchResponse{Devices: p.deviceList()}); err != nil {
				return err
			}
		}
	}
}

func (p *Plugin) GetPreferredAllocation(
	_ context.Context,
	request *pluginapi.PreferredAllocationRequest,
) (*pluginapi.PreferredAllocationResponse, error) {
	response := &pluginapi.PreferredAllocationResponse{}
	for _, containerRequest := range request.ContainerRequests {
		selected, err := preferredDevices(containerRequest)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		response.ContainerResponses = append(response.ContainerResponses, &pluginapi.ContainerPreferredAllocationResponse{DeviceIDs: selected})
	}
	return response, nil
}

func (p *Plugin) Allocate(_ context.Context, request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	response := &pluginapi.AllocateResponse{}
	for _, containerRequest := range request.ContainerRequests {
		ids := append([]string(nil), containerRequest.DevicesIds...)
		sort.Strings(ids)
		for _, id := range ids {
			device, found := p.device(id)
			if !found {
				return nil, status.Errorf(codes.InvalidArgument, "unknown synthetic device %q", id)
			}
			if device.Health != pluginapi.Healthy {
				return nil, status.Errorf(codes.Unavailable, "synthetic device %q is unhealthy", id)
			}
		}
		response.ContainerResponses = append(response.ContainerResponses, &pluginapi.ContainerAllocateResponse{Envs: map[string]string{
			"SYNTHETIC_ACCELERATOR_IDS":      strings.Join(ids, ","),
			"SYNTHETIC_ACCELERATOR_RESOURCE": p.config.ResourceName,
		}})
	}
	return response, nil
}

func (p *Plugin) SetHealth(id, health string) error {
	if health != pluginapi.Healthy && health != pluginapi.Unhealthy {
		return fmt.Errorf("unsupported health %q", health)
	}
	p.mu.Lock()
	device, found := p.devices[id]
	if !found {
		p.mu.Unlock()
		return fmt.Errorf("unknown synthetic device %q", id)
	}
	if device.Health == health {
		p.mu.Unlock()
		return nil
	}
	device.Health = health
	p.mu.Unlock()
	select {
	case p.updates <- struct{}{}:
	default:
	}
	return nil
}

func (p *Plugin) register(ctx context.Context) error {
	kubeletSocket := filepath.Join(p.config.SocketDir, filepath.Base(pluginapi.KubeletSocket))
	connection, err := grpc.NewClient(
		"passthrough:///kubelet-device-plugin",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", kubeletSocket)
		}),
	)
	if err != nil {
		return fmt.Errorf("create kubelet registration client: %w", err)
	}
	defer connection.Close()

	client := pluginapi.NewRegistrationClient(connection)
	if _, err := client.Register(ctx, &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     p.endpoint,
		ResourceName: p.config.ResourceName,
		Options:      &pluginapi.DevicePluginOptions{GetPreferredAllocationAvailable: true},
	}); err != nil {
		return fmt.Errorf("register %s with kubelet: %w", p.config.ResourceName, err)
	}
	return nil
}

func (p *Plugin) deviceList() []*pluginapi.Device {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ids := make([]string, 0, len(p.devices))
	for id := range p.devices {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	devices := make([]*pluginapi.Device, 0, len(ids))
	for _, id := range ids {
		devices = append(devices, clonePluginDevice(p.devices[id]))
	}
	return devices
}

func (p *Plugin) device(id string) (*pluginapi.Device, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	device, found := p.devices[id]
	if !found {
		return nil, false
	}
	return clonePluginDevice(device), true
}

func clonePluginDevice(device *pluginapi.Device) *pluginapi.Device {
	cloned := &pluginapi.Device{ID: device.ID, Health: device.Health}
	if device.Topology != nil {
		cloned.Topology = &pluginapi.TopologyInfo{Nodes: make([]*pluginapi.NUMANode, 0, len(device.Topology.Nodes))}
		for _, node := range device.Topology.Nodes {
			cloned.Topology.Nodes = append(cloned.Topology.Nodes, &pluginapi.NUMANode{ID: node.ID})
		}
	}
	return cloned
}

func preferredDevices(request *pluginapi.ContainerPreferredAllocationRequest) ([]string, error) {
	if request.AllocationSize < 0 || int(request.AllocationSize) > len(request.AvailableDeviceIDs) {
		return nil, fmt.Errorf("allocation size %d exceeds %d available devices", request.AllocationSize, len(request.AvailableDeviceIDs))
	}
	available := make(map[string]struct{}, len(request.AvailableDeviceIDs))
	for _, id := range request.AvailableDeviceIDs {
		available[id] = struct{}{}
	}
	selectedSet := make(map[string]struct{}, request.AllocationSize)
	selected := make([]string, 0, request.AllocationSize)
	mustInclude := append([]string(nil), request.MustIncludeDeviceIDs...)
	sort.Strings(mustInclude)
	for _, id := range mustInclude {
		if _, found := available[id]; !found {
			return nil, fmt.Errorf("required device %q is not available", id)
		}
		if _, duplicate := selectedSet[id]; duplicate {
			continue
		}
		selected = append(selected, id)
		selectedSet[id] = struct{}{}
	}
	if len(selected) > int(request.AllocationSize) {
		return nil, fmt.Errorf("required devices exceed allocation size")
	}
	availableIDs := append([]string(nil), request.AvailableDeviceIDs...)
	sort.Strings(availableIDs)
	for _, id := range availableIDs {
		if len(selected) == int(request.AllocationSize) {
			break
		}
		if _, exists := selectedSet[id]; exists {
			continue
		}
		selected = append(selected, id)
		selectedSet[id] = struct{}{}
	}
	return selected, nil
}

func socketName(resourceName string) string {
	sum := sha256.Sum256([]byte(resourceName))
	return "synthetic-" + hex.EncodeToString(sum[:6]) + ".sock"
}
