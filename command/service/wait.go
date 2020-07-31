package service

import (
	"context"
	"fmt"
	"github.com/docker/cli/cli/command/formatter"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"sort"
	"time"

	"vbom.ml/util/sortorder"

	"github.com/docker/cli/cli"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/opts"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/spf13/cobra"
)

type waitOptions struct {
	quiet    bool
	format   string
	filter   opts.FilterOpt
	interval time.Duration
	timeout  time.Duration
}

func newWaitCommand(dockerCli command.Cli) *cobra.Command {
	options := waitOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "wait [OPTIONS]",
		Short: "Wait for service replication",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(dockerCli, options)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&options.quiet, "quiet", "q", false, "Only display IDs")
	flags.StringVar(&options.format, "format", "", "Pretty-print services using a Go template")
	flags.VarP(&options.filter, "filter", "f", "Filter output based on conditions provided")
	flags.DurationVar(&options.timeout, "timeout", 10*time.Minute, "Max duration to wait")
	flags.DurationVar(&options.interval, "interval", 1*time.Minute, "Interval in which we check the status")

	return cmd
}

func runList(dockerCli command.Cli, options waitOptions) error {
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel()

	client := dockerCli.Client()

	serviceFilters := options.filter.Value()
	services, err := client.ServiceList(ctx, types.ServiceListOptions{Filters: serviceFilters})
	if err != nil {
		return err
	}

	sort.Slice(services, func(i, j int) bool {
		return sortorder.NaturalLess(services[i].Spec.Name, services[j].Spec.Name)
	})

	outputChan := make(chan map[string]WaitInfo)
	doneChan := make(chan bool)

	if len(services) > 0 {
		go func() {
			timeout := time.After(options.timeout)
			ticker := time.Tick(options.interval)

			info, summary, _ := collectServiceStatus(client, ctx, services)
			outputChan <- info
			if summary.Expected <= summary.Running {
				doneChan <- true
				return
			}

			for {
				select {
				case <-timeout:
					info, _, _ := collectServiceStatus(client, ctx, services)
					outputChan <- info

					cancel()
					doneChan <- false
					return
				case <-ticker:
					info, summary, _ := collectServiceStatus(client, ctx, services)
					outputChan <- info

					if summary.Expected <= summary.Running {
						doneChan <- true
						return
					}
				}
			}
		}()
	}

	format := options.format
	if len(format) == 0 {
		if len(dockerCli.ConfigFile().ServicesFormat) > 0 && !options.quiet {
			format = dockerCli.ConfigFile().ServicesFormat
		} else {
			format = formatter.TableFormatKey
		}
	}

	servicesCtx := formatter.Context{
		Output: dockerCli.Out(),
		Format: NewListFormat(format, options.quiet),
	}

	for {
		select {
		case result := <-doneChan:
			if result {
				_, err = dockerCli.Out().Write([]byte("\nAll services booted up\n"))
				if err != nil {
					return err
				}
				return nil
			}
			_, _ = dockerCli.Out().Write([]byte("\n"))
			return fmt.Errorf("timeout reached while waiting for the services")
		case info := <-outputChan:
			err := WaitListFormatWrite(servicesCtx, services, info)
			if err != nil {
				return err
			}
		}
	}
}

type WaitInfo struct {
	ListInfo
	Expected uint64
	Running  uint64
}

type WaitInfoSummary struct {
	Expected uint64
	Running  uint64
}

func collectServiceStatus(client client.APIClient, ctx context.Context, services []swarm.Service) (map[string]WaitInfo, *WaitInfoSummary, error) {
	info := map[string]WaitInfo{}
	summary := &WaitInfoSummary{}

	if len(services) > 0 {
		taskFilter := filters.NewArgs()
		for _, service := range services {
			taskFilter.Add("service", service.ID)
		}

		tasks, err := client.TaskList(ctx, types.TaskListOptions{Filters: taskFilter})
		if err != nil {
			return nil, nil, err
		}

		nodes, err := client.NodeList(ctx, types.NodeListOptions{})
		if err != nil {
			return nil, nil, err
		}

		info, summary = GetServicesWaitStatus(services, nodes, tasks)
	}

	return info, summary, nil
}

// GetServicesStatus returns a map of mode and replicas
func GetServicesWaitStatus(services []swarm.Service, nodes []swarm.Node, tasks []swarm.Task) (map[string]WaitInfo, *WaitInfoSummary) {
	running := map[string]uint64{}
	tasksNoShutdown := map[string]uint64{}

	activeNodes := make(map[string]struct{})
	for _, n := range nodes {
		if n.Status.State != swarm.NodeStateDown {
			activeNodes[n.ID] = struct{}{}
		}
	}

	for _, task := range tasks {
		if task.DesiredState != swarm.TaskStateShutdown {
			tasksNoShutdown[task.ServiceID]++
		}

		if _, nodeActive := activeNodes[task.NodeID]; nodeActive && task.Status.State == swarm.TaskStateRunning {
			running[task.ServiceID]++
		}
	}

	info := map[string]WaitInfo{}
	summary := WaitInfoSummary{}
	for _, service := range services {
		info[service.ID] = WaitInfo{}
		if service.Spec.Mode.Replicated != nil && service.Spec.Mode.Replicated.Replicas != nil {
			if service.Spec.TaskTemplate.Placement != nil && service.Spec.TaskTemplate.Placement.MaxReplicas > 0 {
				info[service.ID] = WaitInfo{
					ListInfo: ListInfo{
						Mode:     "replicated",
						Replicas: fmt.Sprintf("%d/%d (max %d per node)", running[service.ID], *service.Spec.Mode.Replicated.Replicas, service.Spec.TaskTemplate.Placement.MaxReplicas),
					},
					Expected: *service.Spec.Mode.Replicated.Replicas,
					Running:  running[service.ID],
				}
				summary.Running += running[service.ID]
				summary.Expected += *service.Spec.Mode.Replicated.Replicas
			} else {
				info[service.ID] = WaitInfo{
					ListInfo: ListInfo{
						Mode:     "replicated",
						Replicas: fmt.Sprintf("%d/%d", running[service.ID], *service.Spec.Mode.Replicated.Replicas),
					},
					Expected: *service.Spec.Mode.Replicated.Replicas,
					Running:  running[service.ID],
				}
				summary.Running += running[service.ID]
				summary.Expected += *service.Spec.Mode.Replicated.Replicas
			}
		} else if service.Spec.Mode.Global != nil {
			info[service.ID] = WaitInfo{
				ListInfo: ListInfo{
					Mode:     "global",
					Replicas: fmt.Sprintf("%d/%d", running[service.ID], tasksNoShutdown[service.ID]),
				},
				Expected: tasksNoShutdown[service.ID],
				Running:  running[service.ID],
			}
			summary.Running += running[service.ID]
			summary.Expected += tasksNoShutdown[service.ID]
		}
	}

	return info, &summary
}
