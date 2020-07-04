#!/bin/bash

# This script is used to create the actual deployment YAML file based on the YAML template.

set -u
set -e

WORK_DIR=$(dirname $(readlink -f $0))
arch=$(uname -m)

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
# load configs
function load_interface_conf {
    if [ -f "$WORK_DIR/interface_conf" ]; then
        source $WORK_DIR/interface_conf
        echo_info "load interface_conf success"
    else
        echo_error "no interface_conf, please check" && exit 1
    fi
}

# create a file based on the YAML template and replace the variables related to harbor
function create_yml_and_replace_harbor {
    [ $# -ne 1 ] && echo_error "create_yml_and_replace_harbor lack args" && exit 1
    file=$1
    file_tpl=${file}.tpl
    [ -f $file_tpl.$arch ] && file_tpl=${file}.tpl.$arch
    cp $file_tpl $file -f
    sed -i "s!_HARBOR_SECRET_NAME_!$harbor_secret_name!g" $file
    sed -i "s!_HARBOR_IMAGE_ADDR_!$harbor_image_addr!g" $file
    echo_info "create $file success"
}

function init {
    # label node related to gpu (optional)
    # kubectl label node $node1_name gpu=deploy
    # label node related to  monitor/alarm including nodes-server prometheus and alertmanager
    kubectl label node $node1_name monitor=deploy
    # label node related to log including elasticsearch
    kubectl label node $node2_name log=deploy
    # label node related to cluster management including etcd and kube-apiserver
    kubectl label node $node1_name cc=deploy

}

function start {
    init

    # 1) monitor/alarm
    file=$WORK_DIR/gpu-metrics-exporter/gpu-metrics-exporter.yml
    create_yml_and_replace_harbor $file

    file=$WORK_DIR/node-exporter/node-exporter.yml
    create_yml_and_replace_harbor $file

    file=$WORK_DIR/node-agent/node-agent.yml
    create_yml_and_replace_harbor $file
    monitor_server_ip=$(kubectl get node -o wide -l monitor=deploy | grep -w Ready | head -1 | awk '{print $6}')
    nodes_server_addr=http://$monitor_server_ip:8999
    sed -i "s!_NODES_SERVER_ADDR_!$nodes_server_addr!g" $file

    file=$WORK_DIR/nodes-server/nodes-server.yml
    create_yml_and_replace_harbor $file

    file=$WORK_DIR/prometheus/recording-rules.yml
    create_yml_and_replace_harbor $file
    file=$WORK_DIR/prometheus/alerting-rules.yml
    create_yml_and_replace_harbor $file
    file=$WORK_DIR/prometheus/prometheus.yml
    create_yml_and_replace_harbor $file

    file=$WORK_DIR/alertmanager/alertmanager.yml
    create_yml_and_replace_harbor $file

    file=$WORK_DIR/data-query-server/data-query-server.yml
    create_yml_and_replace_harbor $file
    prometheus_addr=http://$monitor_server_ip:8998
    sed -i "s!_PROMETHEUS_ADDR_!$prometheus_addr!g" $file

    # 2) log
    file=$WORK_DIR/fluent-bit/fluent-bit.yml
    create_yml_and_replace_harbor $file
    log_server_ip=$(kubectl get node -o wide -l log=deploy | grep -w Ready | head -1 | awk '{print $6}')
    elasticsearch_host_ip=$log_server_ip
    sed -i "s!_ELASTICSEARCH_HOST_IP_!$elasticsearch_host_ip!g" $file
    sed -i "s!_DOCKER_ROOT_DIR_!$docker_root_dir!g" $file

    file=$WORK_DIR/elasticsearch/elasticsearch.yml
    create_yml_and_replace_harbor $file

    # 3) cluster management
    file=$WORK_DIR/etcd/etcd.yml
    create_yml_and_replace_harbor $file
    cc_server_ip=$(kubectl get node -o wide -l cc=deploy | grep -w Ready | head -1 | awk '{print $6}')
    etcd_host_ip=$cc_server_ip
    sed -i "s!_ETCD_HOST_IP_!$etcd_host_ip!g" $file

    file=$WORK_DIR/kube-apiserver/kube-apiserver.yml
    create_yml_and_replace_harbor $file
    kube_apiserver_host_ip=$cc_server_ip
    sed -i "s!_KUBE_APISERVER_HOST_IP_!$kube_apiserver_host_ip!g" $file
    file=$WORK_DIR/kube-apiserver/cc-crd.yml
    create_yml_and_replace_harbor $file
    file=$WORK_DIR/kube-apiserver/ns.yml
    create_yml_and_replace_harbor $file

    file=$WORK_DIR/ote-cc/ote-cc.yml
    create_yml_and_replace_harbor $file

    file=$WORK_DIR/ote-cm/ote-cm.yml
    create_yml_and_replace_harbor $file

    file=$WORK_DIR/ote-k8s-shim/ote-k8s-shim.yml
    create_yml_and_replace_harbor $file

    # 4) others
    file=$WORK_DIR/tiller/tiller.yml
    create_yml_and_replace_harbor $file

    file=$WORK_DIR/tiller-proxy/tiller-proxy.yml
    create_yml_and_replace_harbor $file

    file=$WORK_DIR/mysql/mysql.yml
    create_yml_and_replace_harbor $file

    file=$WORK_DIR/open-api/open-api.yml
    create_yml_and_replace_harbor $file
    sed -i "s!_HARBOR_API_ADDR_!$harbor_api_addr!g" $file
    sed -i "s!_HARBOR_ADMIN_USER_!$harbor_admin_user!g" $file
    sed -i "s/_HARBOR_ADMIN_PASSWD_/$harbor_admin_passwd/g" $file # password has !, Split with /
    sed -i "s!_REPOSITORY_DOMAIN_!$repository_domain!g" $file
    elasticsearch_addr=http://$log_server_ip:9200
    sed -i "s!_ELASTICSEARCH_ADDR_!$elasticsearch_addr!g" $file

    file=$WORK_DIR/web-frontend-nginx/web-frontend-nginx.yml
    create_yml_and_replace_harbor $file
    web_frontend_nginx_addr=http://$monitor_server_ip:8995
    sed -i "s!_WEB_FRONTEND_NGINX_ADDR_!$web_frontend_nginx_addr!g" $file
    sed -i "s!_REPOSITORY_DOMAIN_!$repository_domain!g" $file

    if [ "$EDGE_AUTONOMY_ENABLE" = "true" ]; then
        file=$WORK_DIR/kube-apiserver/edgenode-crd.yml
        create_yml_and_replace_harbor $file

        file=$WORK_DIR/edge-controller/edge-controller.yml
        create_yml_and_replace_harbor $file
        k8s_svc_ip=$(kubectl get svc | grep -w kubernetes | head -1 | awk '{print $3}')
        k8s_svc_port=$(kubectl get svc | grep -w kubernetes | head -1 | awk '{print $5}' | tr -cd "[0-9]")
        new_ip="https://${k8s_svc_ip}:${k8s_svc_port}"
        cp -f /root/.kube/config $WORK_DIR/edge-controller/kubeconfig
        sed -i "s!https.*!$new_ip!" $WORK_DIR/edge-controller/kubeconfig
        kubectl create secret generic kubeconfig --type=Opaque -n kube-system --from-file=kubeconfig=$WORK_DIR/edge-controller/kubeconfig
    fi

    echo_info "create YAML file success ..."
}


function stop() {
    find $WORK_DIR -name "*.yml" | xargs rm -f
    # delete node labels
    # kubectl label node $node1_name gpu- (optional)
    kubectl label node $node1_name monitor-
    kubectl label node $node2_name log-
    kubectl label node $node1_name cc-

    # delete kubeconfig secret
    if [ "$EDGE_AUTONOMY_ENABLE" = "true" ]; then
        kubectl delete secret -n kube-system kubeconfig
    fi

    echo_info "delete YAML file success ..."
}


cd $WORK_DIR && echo_info "WORK_DIR=$WORK_DIR" && load_interface_conf

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
