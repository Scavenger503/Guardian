package watcher

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	"github.com/Scavenger503/Guardian/internal/config"
	"github.com/Scavenger503/Guardian/internal/notifier"
)

type Watcher struct {
	docker   *client.Client
	cfg      *config.Config
	notifier *notifier.Notifier

	// knownDigests caches the last applied remote digest per image.
	// Prevents the infinite notification loop where Guardian re-detects
	// the same update before Docker's container metadata has refreshed.
	knownDigests map[string]string
}

func New(cfg *config.Config, n *notifier.Notifier) (*Watcher, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}

	return &Watcher{
		docker:       cli,
		cfg:          cfg,
		notifier:     n,
		knownDigests: make(map[string]string),
	}, nil
}

func (w *Watcher) Run(ctx context.Context) {
	log("Guardian is running. Poll interval: %s", w.cfg.PollInterval)

	w.poll(ctx)

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log("Shutdown signal received. Goodbye.")
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *Watcher) poll(ctx context.Context) {
	log("Polling containers for image updates...")

	containers, err := w.docker.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("status", "running")),
	})
	if err != nil {
		log("ERROR: failed to list containers: %v", err)
		return
	}

	for _, c := range containers {
		if val, ok := c.Labels[w.cfg.Label]; !ok || val != "true" {
			continue
		}
		w.checkContainer(ctx, c.ID, c.Image, c.ImageID)
	}
}

func (w *Watcher) checkContainer(ctx context.Context, containerID, imageName, imageID string) {
	debugLog("Checking %s", imageName)

	// Get local digest from the running image.
	info, _, err := w.docker.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		log("ERROR: could not inspect image for %s: %v", imageName, err)
		return
	}

	localDigest := extractDigest(info.RepoDigests, imageName)
	if localDigest == "" {
		debugLog("Could not determine local digest for %s — skipping", imageName)
		return
	}

	// Get remote digest without downloading the full image.
	dist, err := w.docker.DistributionInspect(ctx, imageName, "")
	if err != nil {
		log("ERROR: could not fetch remote digest for %s: %v", imageName, err)
		return
	}

	remoteDigest := string(dist.Descriptor.Digest)

	// Cache hit — we already updated to this digest, skip.
	if cached, ok := w.knownDigests[imageName]; ok && cached == remoteDigest {
		debugLog("%s is up to date (cache hit: %s)", imageName, shortDigest(remoteDigest))
		return
	}

	// Local matches remote — up to date, sync cache.
	if localDigest == remoteDigest {
		debugLog("%s is up to date (%s)", imageName, shortDigest(localDigest))
		w.knownDigests[imageName] = localDigest
		return
	}

	log("Update available for %s: %s → %s", imageName, shortDigest(localDigest), shortDigest(remoteDigest))

	// Notify before pulling.
	w.notifier.UpdateFound(containerName(containerID), imageName, localDigest, remoteDigest)

	// Apply the update.
	if err := w.updateContainer(ctx, containerID, imageName); err != nil {
		log("ERROR: failed to update %s: %v", imageName, err)
		w.notifier.UpdateFailed(containerName(containerID), imageName, err)
		return
	}

	// Cache the new digest immediately to prevent re-detection on next poll.
	w.knownDigests[imageName] = remoteDigest
	log("Successfully updated %s", imageName)
	w.notifier.UpdateApplied(containerName(containerID), imageName)
}

func (w *Watcher) updateContainer(ctx context.Context, containerID, imageName string) error {
	log("Pulling new image: %s", imageName)

	reader, err := w.docker.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("image pull failed: %w", err)
	}
	defer reader.Close()
	io.Copy(io.Discard, reader)

	// Capture current container config before stopping.
	inspect, err := w.docker.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("container inspect failed: %w", err)
	}

	log("Stopping container %s...", inspect.Name)
	timeout := 30
	if err := w.docker.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("container stop failed: %w", err)
	}

	log("Removing old container %s...", inspect.Name)
	if err := w.docker.ContainerRemove(ctx, containerID, container.RemoveOptions{}); err != nil {
		return fmt.Errorf("container remove failed: %w", err)
	}

	log("Creating new container %s...", inspect.Name)
	created, err := w.docker.ContainerCreate(
		ctx,
		inspect.Config,
		inspect.HostConfig,
		nil,
		nil,
		inspect.Name,
	)
	if err != nil {
		return fmt.Errorf("container create failed: %w", err)
	}

	log("Starting container %s...", inspect.Name)
	if err := w.docker.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("container start failed: %w", err)
	}

	return nil
}

func containerName(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func extractDigest(repoDigests []string, imageName string) string {
	base := strings.Split(imageName, ":")[0]
	for _, d := range repoDigests {
		if strings.HasPrefix(d, base+"@") {
			parts := strings.SplitN(d, "@", 2)
			if len(parts) == 2 {
				return parts[1]
			}
		}
	}
	if len(repoDigests) > 0 {
		parts := strings.SplitN(repoDigests[0], "@", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}

func shortDigest(digest string) string {
	if len(digest) > 19 {
		return digest[:19] + "..."
	}
	return digest
}

func log(format string, args ...any) {
	fmt.Printf("[guardian] %s "+format+"\n", append([]any{time.Now().Format("2006-01-02 15:04:05")}, args...)...)
}

func debugLog(format string, args ...any) {
	fmt.Printf("[guardian:debug] %s "+format+"\n", append([]any{time.Now().Format("15:04:05")}, args...)...)
}
