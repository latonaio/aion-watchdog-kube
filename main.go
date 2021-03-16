package main

import (
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

	c, err := msclient.NewKanbanClient(ctx)
	if err != nil {
		errCh <- err
	}

	_, err = c.SetKanban(msName, c.GetProcessNumber())
	if err != nil {
		errCh <- err
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		errCh <- err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		errCh <- err
	}

	go watch(ctx, clientset, errCh, c)

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

func watch(ctx context.Context, clientset *kubernetes.Clientset, errCh chan error, c msclient.MicroserviceClient) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
			if err != nil {
				log.Fatalln("failed to get pods:", err)
			}

			for _, pod := range pods.Items {
				if pod.Status.ContainerStatuses[0].Ready == false {
					if hasWaitingStatusProblem(pod.Status.ContainerStatuses[0].State.Waiting.Reason) &&
						countList[pod.Status.ContainerStatuses[0].Name] <= 3 {
						log.Printf("起動に失敗したpodを検知しました。POD名: %s, Reason: %s",pod.Name,pod.Status.ContainerStatuses[0].State.Waiting.Reason)
						ck := msclient.SetConnectionKey("slack")
						metadata := msclient.SetMetadata(map[string]interface{}{
							"pod_name": pod.Name,
							"status":   pod.Status.ContainerStatuses[0].State.Waiting.Reason,
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
						countList[pod.Status.ContainerStatuses[0].Name] += 1
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
