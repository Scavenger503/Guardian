package watcher

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Scavenger503/Guardian/internal/config"
	"github.com/Scavenger503/Guardian/internal/notifier"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type Watcher struct {
	docker   *client.Client
	config   *config.Config
	notifier *notifier.Notifier
	digests  map[string]string
}

func New(cfg *config.Config, n *notifier.Notifier) (*Watcher, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return &Watcher{
		docker:   cli,
		config:   cfg,
		notifier: n,
		digests:  make(map[string]string),
	}, nil
}

func (w *Watcher) Run(ctx context.Context) {
	log.Println("Guardian started — watching labeled containers")
	w.poll(ctx)
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("Guardian shutting down")
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *Watcher) poll(ctx context.Context) {
	containers, err := w.getLabeledContainers(ctx)
	if err != nil {
		log.Printf("Error listing containers: %v", err)
		return
	}

	for _, c := range containers {
		name := strings.TrimPrefix(c.Names[0], "/")
		imageName := c.Image
		log.Printf("Checking %s (%s)", name, imageName)

		currentDigest, err := w.getLocalDigest(ctx, imageName)
		if err != nil {
			log.Printf("Error getting local digest for %s: %v", name, err)
			continue
		}

		remoteDigest, err := w.getRemoteDigest(ctx, imageName)
		if err != nil {
			log.Printf("Error getting remote digest for %s: %v", name, err)
			continue
		}

		if currentDigest == remoteDigest {
			log.Printf("%s is up to date", name)
			continue
		}

		log.Printf("Update found for %s", name)
		w.notifier.UpdateFound(name, imageName, currentDigest, remoteDigest)

		if err := w.update(ctx, c.ID, name, imageName); err != nil {
			log.Printf("Failed to update %s: %v", name, err)
			w.notifier.UpdateFailed(name, imageName, err)
			continue
		}

		w.notifier.UpdateApplied(name, imageName)
	}
}

func (w *Watcher) getLabeledContainers(ctx context.Context) ([]container.Summary, error) {
	f := filters.NewArgs()
	f.Add("label", w.config.Label+"=true")
	return w.docker.ContainerList(ctx, container.ListOptions{
		Filters: f,
	})
}

func (w *Watcher) getLocalDigest(ctx context.Context, imageName string) (string, error) {
	inspect, _, err := w.docker.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		return "", err
	}
	if len(inspect.RepoDigests) > 0 {
		parts := strings.SplitN(inspect.RepoDigests[0], "@", 2)
		if len(parts) == 2 {
			return parts[1], nil
		}
	}
	return inspect.ID, nil
}

func (w *Watcher) getRemoteDigest(ctx context.Context, imageName string) (string, error) {
	dist, err := w.docker.DistributionInspect(ctx, imageName, "")
	if err != nil {
		return "", err
	}
	return string(dist.Descriptor.Digest), nil
}

func (w *Watcher) update(ctx context.Context, containerID, name, imageName string) error {
	// Pull new image
	out, err := w.docker.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}
	defer out.Close()

	// Inspect container for restart config
	inspect, err := w.docker.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("inspect failed: %w", err)
	}

	// Stop old container
	timeout := 30
	if err := w.docker.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("stop failed: %w", err)
	}

	// Remove old container
	if err := w.docker.ContainerRemove(ctx, containerID, container.RemoveOptions{}); err != nil {
		return fmt.Errorf("remove failed: %w", err)
	}

	// Create new container with same config
	resp, err := w.docker.ContainerCreate(ctx,
		inspect.Config,
		inspect.HostConfig,
		nil, nil,
		name,
	)
	if err != nil {
		return fmt.Errorf("create failed: %w", err)
	}

	// Start new container
	if err := w.docker.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start failed: %w", err)
	}

	return nil
}
