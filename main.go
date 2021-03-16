package main

import (
	"aion-watchdog-kube/config"
	"context"
	"k8s.io/client-go/rest"
	"log"
	"syscall"
	"time"

	"bitbucket.org/latonaio/aion-core/pkg/go-client/msclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const msName = "aion-watchdog-kube"

var whitelist = []string{
	"ContainerCreating",
	"PodInitializing",
}

var countList = map[string]int{}

func main() {
	errCh := make(chan error, 1)
	quiteCh := make(chan syscall.Signal, 1)

	ctx, cancel := context.WithCancel(context.Background())
	cfg,err := config.New()
	if err != nil {
		errCh <- err
	}

	c, err := msclient.NewKanbanClient(ctx)
	if err != nil {
		errCh <- err
	}

	_, err = c.SetKanban(msName, c.GetProcessNumber())
	if err != nil {
		errCh <- err
	}

	kubecfg, err := rest.InClusterConfig()
	if err != nil {
		errCh <- err
	}

	clientset, err := kubernetes.NewForConfig(kubecfg)
	if err != nil {
		errCh <- err
	}

	go watch(ctx, clientset, errCh, c,cfg)

loop:
	for {
		select {
		case err := <-errCh:
			log.Print(err)
			break loop
		case <-quiteCh:
			cancel()
		}
	}
}

func watch(ctx context.Context, clientset *kubernetes.Clientset, errCh chan error, c msclient.MicroserviceClient, cfg *config.Config) {
	ticker := time.NewTicker(time.Duration(cfg.WatchPeriod) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
			if err != nil {
				errCh <- err
				return
			}

			for _, pod := range pods.Items {
				for _,status := range pod.Status.ContainerStatuses {
					if status.Ready == false {
						if hasWaitingStatusProblem(status.State.Waiting.Reason) &&
							countList[status.Name] <= cfg.MaxAlertNum-1 {
							log.Printf("起動に失敗したpodを検知しました。POD名: %s, Reason: %s", pod.Name, status.State.Waiting.Reason)
							ck := msclient.SetConnectionKey("slack")
							metadata := msclient.SetMetadata(map[string]interface{}{
								"pod_name": pod.Name,
								"status":   status.State.Waiting.Reason,
								"level":    "warning",
							})
							req, err := msclient.NewOutputData(ck, metadata)
							if err != nil {
								errCh <- err
								return
							}
							err = c.OutputKanban(req)
							if err != nil {
								errCh <- err
								return
							}
							log.Printf("kanbanデータの送信に成功しました")
							countList[status.Name] += 1
						}
					}
				}
			}
		}
	}

}

func hasWaitingStatusProblem(status string) bool {
	for _, v := range whitelist {
		if v == status {
			return false
		}
	}
	return true
}
