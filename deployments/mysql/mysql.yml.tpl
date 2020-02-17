apiVersion: v1
kind: Service
metadata:
  name: ote-mysql
  namespace: kube-system
  labels:
    app: ote-mysql
  annotations:
    service.alpha.kubernetes.io/tolerate-unready-endpoints: "true"
spec:
  clusterIP: None
  selector:
    app: ote-mysql
  ports:
  - port: 8306
    name: ote-mysql
    targetPort: 8306
---

apiVersion: v1
kind: ConfigMap
metadata:
  name: db-sql2
  namespace: kube-system
data:
  entrypoint.sh: |
    #!/bin/bash
    # Taken from the official mysql-repo
    # And changed for simplification of course :)
    # I.e. DATADIR is always /var/lib/mysql
    # We don't force the usage of MYSQL_ALLOW_EMPTY_PASSWORD
    # erkan.yanar@linsenraum.de
    set -e
    set -x
    # Check ENV (MYSQL_) and stop if they are not known variables
    # TODO


    tempSqlFile='/tmp/mysql-first-time.sql'
    MYSQL_ROOT_PASSWORD=123456
    if [ ! -d "/var/lib/mysql/mysql" ]; then

       echo 'Running mysql_install_db ...'
       mysql_install_db --datadir=/var/lib/mysql
       echo 'Finished mysql_install_db'

       # These statements _must_ be on individual lines, and _must_ end with
       # semicolons (no line breaks or comments are permitted).
       # TODO proper SQL escaping on ALL the things D:
       cat /home/db.sql >> "$tempSqlFile"

       cat >> "$tempSqlFile" <<-EOSQL
    -- What's done in this file shouldn't be replicated
    --  or products like mysql-fabric won't work
    SET @@SESSION.SQL_LOG_BIN=0;

    DELETE FROM mysql.user ;
    CREATE USER 'root'@'%' IDENTIFIED BY '${MYSQL_ROOT_PASSWORD}' ;
    GRANT ALL ON *.* TO 'root'@'%' WITH GRANT OPTION ;
    DROP DATABASE IF EXISTS test ;
    EOSQL


        if [ "$MYSQL_DATABASE" ]; then
            echo "CREATE DATABASE IF NOT EXISTS \`$MYSQL_DATABASE\` ;" >> "$tempSqlFile"
        fi

        if [ "$MYSQL_USER" -a "$MYSQL_PASSWORD" ]; then
            echo "CREATE USER '$MYSQL_USER'@'%' IDENTIFIED BY '$MYSQL_PASSWORD' ;" >> "$tempSqlFile"
            echo "CREATE USER '$MYSQL_USER'@'localhost' IDENTIFIED BY '$MYSQL_PASSWORD' ;" >> "$tempSqlFile"
        fi
        if [ "$MYSQL_USER" -a ! "$MYSQL_PASSWORD" ]; then
            echo "CREATE USER '$MYSQL_USER'@'%'  ;"         >> "$tempSqlFile"
            echo "CREATE USER '$MYSQL_USER'@'localhost'  ;" >> "$tempSqlFile"
        fi

        if [ "$MYSQL_USER" -a  "$MYSQL_DATABASE"  ]; then
            echo "GRANT ALL ON \`$MYSQL_DATABASE\`.* TO '$MYSQL_USER'@'%' ;" >> "$tempSqlFile"
            echo "GRANT ALL ON \`$MYSQL_DATABASE\`.* TO '$MYSQL_USER'@'localhost' ;" >> "$tempSqlFile"
        fi

        echo 'FLUSH PRIVILEGES ;' >> "$tempSqlFile"
        set -- "$@" --init-file="$tempSqlFile"
        sed -i "s/skip-grant-tables/#skip-grant-tables/g" /etc/mysql/my-galera.cnf
    fi
    echo 'port=8306' >> /etc/mysql/my-galera.cnf
    echo @a
    set -- mysqld "$@"
    chown -R mysql:mysql /var/lib/mysql
    chown -R mysql:mysql /var/run/mysqld
    echo "Checking to upgrade the schema"
    echo "A failed upgrade is ok when there was no upgrade"
    # mysql_upgrade || true
    exec "$@"

  db.sql: |
    CREATE DATABASE sys_ote_manage_platform;
    USE sys_ote_manage_platform;
    SET names utf8;

    CREATE TABLE IF NOT EXISTS `tb_system_config` (
      `key` varchar(64) NOT NULL COMMENT '键',
      `value` varchar(64) NOT NULL COMMENT '值',
      PRIMARY KEY (`key`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='系统配置';

    CREATE TABLE IF NOT EXISTS `tb_business_info` (
      `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增ID',
      `name` varchar(64) NOT NULL COMMENT '业务名',
      `namespace` varchar(64) NOT NULL DEFAULT '' COMMENT '命名空间',
      `user_id` bigint(20) unsigned NOT NULL COMMENT '申请用户id',
      `introduce` varchar(512) NOT NULL COMMENT '业务介绍',
      `objective` varchar(512) NOT NULL COMMENT '需要资源',
      `scale` varchar(512) NOT NULL COMMENT '业务规模',
      `comment` varchar(512) NOT NULL DEFAULT '' COMMENT '评审意见',
      `status` tinyint NOT NULL DEFAULT 0 COMMENT '业务状态',
      `update_time` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
      `create_time` TIMESTAMP NOT NULL COMMENT '创建时间',
      PRIMARY KEY (`id`),
      UNIQUE KEY (`name`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='业务列表';

    CREATE TABLE IF NOT EXISTS `tb_ote_web_users` (
      `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
      `uid` varchar(50) COLLATE utf8_unicode_ci NOT NULL COMMENT '用户id',
      `namespace` varchar(64) COLLATE utf8_unicode_ci DEFAULT '' COMMENT '命名空间',
      `user_name` varchar(50) COLLATE utf8_unicode_ci DEFAULT '' COMMENT '用户名',
      `display_name` varchar(50) COLLATE utf8_unicode_ci DEFAULT '' COMMENT '用户display_name',
      `real_name` varchar(50) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '真实姓名',
      `password` varchar(60) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '密码',
      `email` varchar(255) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '邮箱',
      `phone` varchar(11) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '手机号',
      `status` tinyint(4) unsigned NOT NULL DEFAULT '3' COMMENT '状态: 0:待审核,1:生效,2:审核不通过,3:禁用，',
      `is_admin` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT '是否管理员',
      `role` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '角色:0:未授权,1:普通,2:管理员,3:超级管理员',
      `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
      `created_at` timestamp NOT NULL COMMENT '创建时间',
      PRIMARY KEY (`id`),
      UNIQUE KEY `tb_ote_web_users_uid_unique` (`uid`) USING BTREE,
      UNIQUE KEY `tb_ote_web_users_phone_unique` (`phone`) USING BTREE,
      UNIQUE KEY `tb_ote_web_users_user_name_unique` (`user_name`) USING BTREE
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci COMMENT='用户信息表';

    CREATE TABLE IF NOT EXISTS `tb_ote_web_repository_users` (
      `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
      `namespace` varchar(64) NOT NULL DEFAULT '' COMMENT '命名空间',
      `repository_uid` bigint(20) unsigned DEFAULT '0' COMMENT '镜像仓库用户id',
      `repository_username` varchar(50) COLLATE utf8_unicode_ci NOT NULL COMMENT '镜像仓库用户名',
      `repository_email` varchar(255) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '镜像仓库用户邮箱',
      `repository_password` varchar(255) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '镜像仓库用户密码',
      `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
      `created_at` timestamp NOT NULL COMMENT '创建时间',
      PRIMARY KEY (`id`),
      UNIQUE KEY (`namespace`) USING BTREE
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci COMMENT='harbor仓库用户表';

    CREATE TABLE IF NOT EXISTS `tb_ote_web_third_repository` (
      `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增id',
      `namespace` varchar(64) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '用户namespace',
      `repository_id` varchar(50) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '第三方仓库id',
      `repository_url` varchar(500) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '第三方仓库url',
      `repository_username` varchar(50) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '第三方仓库用户名',
      `repository_password` varchar(255) COLLATE utf8_unicode_ci NOT NULL COMMENT '第三方仓库密码',
      `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
      `created_at` timestamp NOT NULL COMMENT '创建时间',
      PRIMARY KEY (`id`),
      UNIQUE KEY (`namespace`, `repository_id`) USING BTREE
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci COMMENT='第三方仓库表';

    CREATE TABLE IF NOT EXISTS `tb_ote_cluster_label` (
      `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增id',
      `cluster_name` varchar(64) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '集群名',
      `label` varchar(64) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '集群标签',
      `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
      `created_at` timestamp NOT NULL COMMENT '创建时间',
      PRIMARY KEY (`id`),
      UNIQUE KEY (`cluster_name`, `label`) USING BTREE
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci COMMENT='集群标签';

    CREATE TABLE IF NOT EXISTS `tb_ote_node_label` (
      `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增id',
      `cluster_name` varchar(64) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '集群名',
      `node_name` varchar(64) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '机器名',
      `label` varchar(64) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '机器标签',
      `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
      `created_at` timestamp NOT NULL COMMENT '创建时间',
      PRIMARY KEY (`id`),
      UNIQUE KEY (`cluster_name`, `node_name`, `label`) USING BTREE
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci COMMENT='机器标签';

    CREATE TABLE IF NOT EXISTS `tb_app_info` (
      `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增ID',
      `namespace` varchar(64) NOT NULL COMMENT 'namespace',
      `app_name` varchar(64) NOT NULL COMMENT '服务名',
      `main_version` varchar(32) NOT NULL COMMENT '大版本号',
      `version_count` int(11) NOT NULL COMMENT '小版本号',
      `image` varchar(256) NOT NULL COMMENT '镜像地址',
      `repository_id` varchar(64) NOT NULL DEFAULT '' COMMENT '仓库id',
      `port` varchar(1024) NOT NULL DEFAULT '0' COMMENT '端口',
      `env` varchar(1024) NOT NULL DEFAULT '' COMMENT '环境变量',
      `volume` varchar(1024) NOT NULL DEFAULT '' COMMENT '挂载卷',
      `dependence` varchar(1024) NOT NULL DEFAULT '' COMMENT '通信环境变量',
      `replicas` int(10) NOT NULL DEFAULT '1' COMMENT '默认份数',
      `command` varchar(512) NOT NULL DEFAULT '' COMMENT '启动命令',
      `deploy_type` tinyint(4) NOT NULL DEFAULT '0' COMMENT '部署方式: 0:deployment, 1:daemonset',
      `gpu` int(10) NOT NULL DEFAULT '0',
      `request_cpu` int(10) NOT NULL DEFAULT '80',
      `request_mem` int(10) NOT NULL DEFAULT '4096',
      `limit_cpu` int(10) NOT NULL DEFAULT '80',
      `limit_mem` int(10) NOT NULL DEFAULT '4096',
      `min_replicas` int(10) NOT NULL DEFAULT '1',
      `max_replicas` int(10) NOT NULL DEFAULT '5',
      `is_hpa` tinyint(4) NOT NULL DEFAULT '0' COMMENT '是否动态伸缩：0：否，1：是',
      `hpa_target_cpu` int(10) NOT NULL DEFAULT '80',
      `hpa_target_mem` int(10) NOT NULL DEFAULT '4096',
      `min_ready_seconds` int(10) NOT NULL DEFAULT '10',
      `max_surge` int(10) NOT NULL DEFAULT '1',
      `max_unavailable` int(10) NOT NULL DEFAULT '0',
      `status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '状态:0:初始化,1:成功,2:失败,3:逻辑删除,4:物理删除',
      `update_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
      `create_time` timestamp NOT NULL COMMENT '创建时间',
      PRIMARY KEY (`id`),
      UNIQUE KEY `namespace` (`namespace`,`app_name`,`main_version`,`version_count`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='应用包列表';

    CREATE TABLE IF NOT EXISTS `tb_deploy_info` (
      `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增ID',
      `name` varchar(128) NOT NULL COMMENT '名字',
      `namespace` varchar(64) NOT NULL COMMENT '命名空间',
      `app_name` varchar(64) NOT NULL COMMENT '服务名',
      `version` varchar(48) NOT NULL COMMENT '版本号',
      `cluster` varchar(64) NOT NULL COMMENT '集群名',
      `node_label` varchar(64) NOT NULL DEFAULT 'all',
      `status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '部署状态',
      `editable` tinyint(4) NOT NULL DEFAULT '1' COMMENT '能否编辑: 0:否，1:是',
      `running` tinyint(4) NOT NULL DEFAULT '0' COMMENT '是否生效中: 0:否，1:是',
      `deploy_type` tinyint(4) NOT NULL DEFAULT '0' COMMENT '操作类型: 0:新增,1:升级,2:回滚,3:删除',
      `comment` varchar(256) NOT NULL DEFAULT '' COMMENT '备注',
      `audit_comment` varchar(256) NOT NULL DEFAULT '' COMMENT '评审备注',
      `error_message` varchar(256) NOT NULL DEFAULT '' COMMENT '错误信息',
      `update_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
      `create_time` timestamp NOT NULL COMMENT '创建时间',
      `execute_time` timestamp NOT NULL COMMENT '执行时间',
      PRIMARY KEY (`id`),
      KEY `name` (`namespace`,`name`),
      KEY `namespace` (`namespace`,`app_name`,`cluster`,`version`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='部署历史';

    CREATE TABLE IF NOT EXISTS tb_domain_info (
      `id`       BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增ID',
      `namespace`  varchar(64) NOT NULL COMMENT '用户ID',
      `domain`   varchar(128) NOT NULL COMMENT '域名',
      `used_count`   int NOT NULL DEFAULT 0 COMMENT '使用次数',
      `update_time`  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
      `create_time`  TIMESTAMP NOT NULL COMMENT '创建时间',
      PRIMARY KEY(`id`),
      KEY(`namespace`),
      UNIQUE KEY(`domain`)
    ) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8 COMMENT = '域名';

    CREATE TABLE IF NOT EXISTS tb_ingress_info (
      `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增ID',
      `namespace`  varchar(64) NOT NULL COMMENT '用户ID',
      `domain`     varchar(128) NOT NULL COMMENT '域名',
      `uri`        varchar(128) NOT NULL COMMENT 'uri',
      `is_rewrite` int8 NOT NULL DEFAULT 0 COMMENT '',
      `deploy_name`  varchar(64) NOT NULL COMMENT '部署名',
      `status`       tinyint(4) NOT NULL DEFAULT '0' COMMENT '部署状态',
      `unique_key`   varchar(64) NOT NULL DEFAULT 'unique key',
      `update_time`  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
      `create_time`  TIMESTAMP NOT NULL COMMENT '创建时间',
      PRIMARY KEY(`id`),
      UNIQUE KEY(`namespace`,`unique_key`)
    ) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8 COMMENT = '接入规则';

    CREATE TABLE  IF NOT EXISTS `tb_halo_service` (
      `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增ID',
      `service_name` varchar(64) NOT NULL COMMENT '用户ID',
      `package`     varchar(64) NOT NULL COMMENT '应用包',
      `update_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
      `create_time` timestamp NOT NULL COMMENT '创建时间',
      PRIMARY KEY (`id`),
      UNIQUE KEY `service_name` (`service_name`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='halo服务表';

    CREATE TABLE IF NOT EXISTS tb_halo_package (
      id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增ID',
      package_name varchar(64) NOT NULL COMMENT '用户ID',
      reserve_num int NOT NULL DEFAULT 2 COMMENT '保存数量',
      source_type varchar(16) NOT NULL COMMENT '来源类型',
      update_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
      create_time TIMESTAMP NOT NULL COMMENT '创建时间',
      PRIMARY KEY(`id`),
      UNIQUE KEY(`package_name`)
    ) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8 COMMENT = 'halo应用包表';

    CREATE TABLE IF NOT EXISTS tb_halo_command (
      id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '自增ID',
      name varchar(64) NOT NULL COMMENT 'jobName',
      job_id varchar(50) NOT NULL COMMENT 'jobID',
      command_info text NOT NULL COMMENT '任务内容, base64加json',
      create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
      PRIMARY KEY(`id`),
      UNIQUE KEY `name` (`name`)
    ) ENGINE = InnoDB DEFAULT CHARACTER SET = utf8 COMMENT = 'halo命令表';

    CREATE TABLE IF NOT EXISTS `tb_alert_info` (
      `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '自增id',
      `instance` varchar(100) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '告警机器名',
      `ipaddr` varchar(20) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '告警ip',
      `alert_name` varchar(100) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '监控策略',
      `alert_user` varchar(50) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '告警用户',
      `limit_value` varchar(10) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '告警阈值',
      `current_value` varchar(20) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '当前阈值',
      `description` varchar(1000) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '告警描述',
      `generator_url` varchar(500) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '详细情况',
      `status` varchar(10) COLLATE utf8_unicode_ci NOT NULL DEFAULT '' COMMENT '当前状态',
      `update_time` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
      `create_time` TIMESTAMP NOT NULL COMMENT '创建时间',
      PRIMARY KEY (`id`),
      UNIQUE KEY(`instance`, `alert_name`,`create_time`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci  COMMENT = '告警信息表';
---

apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ote-mysql
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: ote-mysql
  serviceName: ote-mysql
  replicas: 1
  template:
    metadata:
      labels:
        app: ote-mysql
    spec:
      imagePullSecrets:
      - name: registry.dcdn.baidu.com.key
      initContainers:
      - name: install
        image:  _HARBOR_IMAGE_ADDR_/galera-install:0.1
        imagePullPolicy: IfNotPresent
        args:
        - "--work-dir=/work-dir"
        volumeMounts:
        - name: workdir
          mountPath: "/work-dir"
        - name: config
          mountPath: "/etc/mysql"
      - name: bootstrap
        image:  _HARBOR_IMAGE_ADDR_/debian:jessie
        imagePullPolicy: IfNotPresent
        command:
        - "/work-dir/peer-finder"
        args:
        - -on-start="/work-dir/on-start.sh"
        - "-service=ote-mysql"
        env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        volumeMounts:
        - name: workdir
          mountPath: "/work-dir"
        - name: config
          mountPath: "/etc/mysql"
      affinity:
        # nodeAffinity:
        #   requiredDuringSchedulingIgnoredDuringExecution:
        #     nodeSelectorTerms:
        #     - matchExpressions:
        #       - key: idc
        #         operator: In
        #         values:
        #         - ote
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: app
                operator: In
                values:
                - ote-mysql
            topologyKey: "kubernetes.io/hostname"
      containers:
      - name: mysql
        image:  _HARBOR_IMAGE_ADDR_/mysql-galera:e2e
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8306
          name: mysql
        - containerPort: 4444
          name: sst
        - containerPort: 4567
          name: replication
        - containerPort: 4568
          name: ist
        command:
        - /home/entrypoint.sh
        - --defaults-file=/etc/mysql/my-galera.cnf
        - --user=root
        readinessProbe:
          # TODO: If docker exec is buggy just use gcr.io/google_containers/mysql-healthz:1.0
          exec:
            command:
            - sh
            - -c
            - "mysql -u root -p123456 -e 'show databases;'"
          initialDelaySeconds: 15
          timeoutSeconds: 5
          successThreshold: 2
        volumeMounts:
        - name: datadir
          mountPath: /var/lib/
        - name: config
          mountPath: /etc/mysql
        - name: db-sql2
          mountPath: /home
      volumes:
      - name: config
        emptyDir: {}
      - name: workdir
        emptyDir: {}
      - name: datadir
        hostPath:
          path: /home/work/ote/mysql-data/
      - name: db-sql2
        configMap:
          defaultMode: 0555
          name: db-sql2
