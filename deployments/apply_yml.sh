#!/bin/bash

# This script is used to create or delete all modules.

set -u
set -e

WORK_DIR=$(dirname $(readlink -f $0))

function echo_error {
    [ $# -eq 0 ] && return
    input="$@"
    DATE_N=`date "+%Y-%m-%d %H:%M:%S"`
    echo -e "\E[1;31m ${DATE_N} [ERROR] $input \E[0m"
}
function echo_info {
    [ $# -eq 0 ] && return
    input="$@"
    DATE_N=`date "+%Y-%m-%d %H:%M:%S"`
    echo -e "\E[1;32m ${DATE_N} [INFO] $input \E[0m"
}

function start {
    # 1) monitor/alarm
    kubectl apply -f gpu-metrics-exporter/gpu-metrics-exporter.yml
    kubectl apply -f node-exporter/node-exporter.yml
    kubectl apply -f node-agent/node-agent.yml
    kubectl apply -f nodes-server/nodes-server.yml
    kubectl apply -f prometheus/recording-rules.yml
    kubectl apply -f prometheus/alerting-rules.yml
    kubectl apply -f prometheus/prometheus.yml
    kubectl apply -f alertmanager/alertmanager.yml
    kubectl apply -f data-query-server/data-query-server.yml
    # 2) log
    kubectl apply -f fluent-bit/fluent-bit.yml
    kubectl apply -f elasticsearch/elasticsearch.yml

    # 3) cluster management
    kubectl apply -f etcd/etcd.yml
    kubectl apply -f kube-apiserver/kube-apiserver.yml
    kube_apiserver_host_ip=$(kubectl get node -o wide -l cc=deploy | grep -w Ready | head -1 | awk '{print $6}')
    kube_apiserver_port=9082
    while true; do
        nc -vz -w 3 $kube_apiserver_host_ip $kube_apiserver_port && [ $? -eq 0 ] && break
        echo_info "sleep 5s and wait for kube-apiserver ready" && sleep 5
    done

    kubectl apply -f kube-apiserver/ns.yml -s $kube_apiserver_host_ip:$kube_apiserver_port
    kubectl apply -f kube-apiserver/cc-crd.yml -s $kube_apiserver_host_ip:$kube_apiserver_port
    kubectl apply -f tiller/tiller.yml
    kubectl apply -f tiller-proxy/tiller-proxy.yml
    kubectl create configmap edge-kube-config -n kube-system --from-file=/root/.kube/config
    kubectl apply -f ote-cc/ote-cc.yml
    echo_info "sleep 10s and wait for ote-cc ready" && sleep 10
    kubectl apply -f ote-cm/ote-cm.yml
    echo_info "sleep 10s and wait for ote-cm ready" && sleep 10
    kubectl apply -f ote-k8s-shim/ote-k8s-shim.yml

    # 4) others
    kubectl apply -f mysql/mysql.yml
    kubectl apply -f open-api/open-api.yml
    kubectl apply -f web-frontend-nginx/web-frontend-nginx.yml

    echo_info "kubectl apply all yml success..."
}

function stop {
    # 1) monitor/alarm
    kubectl delete -f gpu-metrics-exporter/gpu-metrics-exporter.yml
    kubectl delete -f node-exporter/node-exporter.yml
    kubectl delete -f node-agent/node-agent.yml
    kubectl delete -f nodes-server/nodes-server.yml
    kubectl delete -f prometheus/recording-rules.yml
    kubectl delete -f prometheus/alerting-rules.yml
    kubectl delete -f prometheus/prometheus.yml
    kubectl delete -f alertmanager/alertmanager.yml
    kubectl delete -f data-query-server/data-query-server.yml
    # 2) log
    kubectl delete -f fluent-bit/fluent-bit.yml
    kubectl delete -f elasticsearch/elasticsearch.yml

    # 3) cluster management
    kubectl delete -f etcd/etcd.yml
    kubectl delete -f kube-apiserver/kube-apiserver.yml
    kubectl delete -f tiller/tiller.yml
    kubectl delete -f tiller-proxy/tiller-proxy.yml
    kubectl delete configmap edge-kube-config -n kube-system
    kubectl delete -f ote-cc/ote-cc.yml
    kubectl delete -f ote-cm/ote-cm.yml
    kubectl delete -f ote-k8s-shim/ote-k8s-shim.yml

    # 4) others
    kubectl delete -f mysql/mysql.yml
    kubectl delete -f open-api/open-api.yml
    kubectl delete -f web-frontend-nginx/web-frontend-nginx.yml
    echo_info "kubectl delete all yml success..."
}

cd $WORK_DIR && echo_info "WORK_DIR=$WORK_DIR"

case $1 in
start)
    start
;;
stop)
    stop
;;
restart)
    stop
    start
;;
*)
    echo_error "Usage ./$0 start|stop|restart" && exit 1
;;
esac