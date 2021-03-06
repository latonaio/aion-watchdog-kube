# aion-watchdog-kube
aion-watchdog-kubeは、kubernetesのWatch APIを使い、podの死活監視を行うマイクロサービスです。

## 動作環境
aion-watchdog-kubeは、以下の環境環境を前提とします。   
- OS: Linux   
- CPU: Intel64/AMD64/ARM64   

最低限スペック  
- CPU: 2 core    
- memory: 4 GB    

## 起動方法
1. docker imageのビルド
```
$ cd ~/path/to/aion-watchdog-kube
$ bash docker-build.sh
```
2. project.yamlに次の設定を追加してください。
```yaml
aion-watchdog-kube:
    startup: yes
    always: yes
    privileged: yes
    serviceAccount: kubeのservice-account
    nextService:
      - default:
        name: XXX //監視結果の伝達先マイクロサービス
```
  

## I/O
### Input  
KubernetesのWatch APIを使用し、2秒ごとに他のpodのデプロイ状態を常時監視します。
  
### Output  
kanbanデータを送信します。
送信するパラメーターは下記です。
```
pod_name string
status string
level string
```

## 監視処理について
2秒間隔でpodの死活監視を行います。

待機状態のpodを発見した時、whiteListで指定したReason以外のReasonで待機中の場合はkanbanにpod名とstatusを送ります。

statusにはReasonが入ります。