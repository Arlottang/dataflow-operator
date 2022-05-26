package controllers

import (
	"context"
	dataflowv1 "github.com/StepOnce7/dataflow-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *DataflowEngineReconciler) ReconcileMysqlCluster(ctx context.Context, instance *dataflowv1.DataflowEngine, req ctrl.Request) (ctrl.Result, error) {

	logg := log.FromContext(ctx)

	var cfm corev1.ConfigMap
	cfm.Name = "mysql"
	cfm.Namespace = instance.Namespace

	or, err := ctrl.CreateOrUpdate(ctx, r.Client, &cfm, func() error {
		FrameConfigMap(&cfm)
		return controllerutil.SetControllerReference(instance, &cfm, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create mysql configMap error")
		return ctrl.Result{}, err
	}
	logg.Info("CreateOrUpdate", "Mysql ConfigMap", or)

	var headLessSvc corev1.Service
	headLessSvc.Name = "mysql"
	headLessSvc.Namespace = instance.Namespace

	or, err = ctrl.CreateOrUpdate(ctx, r.Client, &headLessSvc, func() error {
		FrameHeadlessService(instance, &headLessSvc)
		return controllerutil.SetControllerReference(instance, &headLessSvc, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create mysql headless service error")
		return ctrl.Result{}, err
	}
	logg.Info("CreateOrUpdate", "Mysql Headless Service", or)

	var svc corev1.Service
	svc.Name = "mysql-read"
	svc.Namespace = instance.Namespace

	or, err = ctrl.CreateOrUpdate(ctx, r.Client, &svc, func() error {
		FrameService(instance, &svc)
		return controllerutil.SetControllerReference(instance, &svc, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create mysql service error")
		return ctrl.Result{}, err
	}

	logg.Info("CreateOrUpdate", "Mysql Service", or)

	var sts appsv1.StatefulSet
	sts.Name = "mysql"
	sts.Namespace = instance.Namespace
	or, err = ctrl.CreateOrUpdate(ctx, r.Client, &sts, func() error {
		FrameStatefulSet(instance, &sts)
		return controllerutil.SetControllerReference(instance, &sts, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create mysql statefulSet error")
		return ctrl.Result{}, err
	}

	logg.Info("CreateOrUpdate", "Mysql StatefulSet", or)

	logg.Info("frame standalone reconcile end", "reconcile", "success")

	return ctrl.Result{}, nil
}

func FrameConfigMap(cfm *corev1.ConfigMap) {
	cfm.Labels = map[string]string{
		MysqlClusterCommonLabelKey: "mysql",
	}

	cfm.Data = map[string]string{
		"primary.cnf": `# Apply this config only on the primary.	
	[mysqld]
    log-bin`,
		"replica.cnf": `# Apply this config only on replicas.
	[mysqld]
    super-read-only`,
	}
}

func FrameHeadlessService(de *dataflowv1.DataflowEngine, svc *corev1.Service) {
	svc.Labels = map[string]string{
		MysqlClusterCommonLabelKey: "mysql",
	}

	svc.Spec = corev1.ServiceSpec{
		ClusterIP: corev1.ClusterIPNone,
		Selector: map[string]string{
			MysqlClusterLabelKey: de.Spec.FrameStandalone.Name,
		},
		Ports: []corev1.ServicePort{
			{
				Name: "mysql",
				Port: de.Spec.FrameStandalone.Port,
			},
		},
	}
}

func FrameService(de *dataflowv1.DataflowEngine, svc *corev1.Service) {
	svc.Labels = map[string]string{
		MysqlClusterCommonLabelKey: "mysql",
	}

	svc.Spec = corev1.ServiceSpec{
		Selector: map[string]string{
			MysqlClusterLabelKey: de.Spec.FrameStandalone.Name,
		},
		Ports: []corev1.ServicePort{
			{
				Name: "mysql",
				Port: de.Spec.FrameStandalone.Port,
			},
		},
	}
}

func FrameStatefulSet(de *dataflowv1.DataflowEngine, sts *appsv1.StatefulSet) {
	sts.Labels = map[string]string{
		MysqlClusterCommonLabelKey: "mysql",
	}

	sts.Spec = appsv1.StatefulSetSpec{
		Replicas: de.Spec.FrameStandalone.Size,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				MysqlClusterLabelKey: de.Spec.FrameStandalone.Name,
			},
		},
		ServiceName: "mysql",
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: de.Namespace,
				Labels: map[string]string{
					MysqlClusterCommonLabelKey: "mysql",
					MysqlClusterLabelKey:       de.Spec.FrameStandalone.Name,
				},
			},
			Spec: corev1.PodSpec{
				InitContainers: newFrameInitContainers(de),
				Containers:     newFrameContainers(de),
				Volumes: []corev1.Volume{
					{
						Name: "conf",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{
								Medium: corev1.StorageMediumDefault,
							},
						},
					},
					{
						Name: "config-map",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "mysql",
								},
							},
						},
					},
				},
			},
		},
		VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "data",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
			},
		},
	}
}

func newFrameInitContainers(de *dataflowv1.DataflowEngine) []corev1.Container {

	return []corev1.Container{
		{
			Name:            "init-mysql",
			Image:           de.Spec.FrameStandalone.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         loadInitMysqlCommand(),
			Env: []corev1.EnvVar{
				{
					Name: "POD_HOSTNAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "conf",
					MountPath: "/mnt/conf.d",
				},
				{
					Name:      "config-map",
					MountPath: "/mnt/config-map",
				},
			},
		},
		{
			Name:            "clone-mysql",
			Image:           "gcr.io/google-samples/xtrabackup:1.0",
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         loadCloneMysqlCommand(),
			Env: []corev1.EnvVar{
				{
					Name: "POD_HOSTNAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "data",
					MountPath: "/var/lib/mysql",
					SubPath:   "mysql",
				},
				{
					Name:      "conf",
					MountPath: "/etc/mysql/conf.d",
				},
			},
		},
	}
}

func loadInitMysqlCommand() []string {
	return []string{
		"bash", "-c",
		`set -ex
          [[ ${POD_HOSTNAME}	 =~ -([0-9]+)$ ]] || exit 1
          ordinal=${BASH_REMATCH[1]}
          echo [mysqld] > /mnt/conf.d/server-id.cnf
          echo server-id=$((100 + $ordinal)) >> /mnt/conf.d/server-id.cnf
          if [[ $ordinal -eq 0 ]]; then
            cp /mnt/config-map/primary.cnf /mnt/conf.d/
          else
            cp /mnt/config-map/replica.cnf /mnt/conf.d/
          fi`,
	}
}

func loadCloneMysqlCommand() []string {
	return []string{
		"bash", "-c",
		`set -ex
          [[ -d /var/lib/mysql/mysql ]] && exit 0
          [[ ${POD_HOSTNAME} =~ -([0-9]+)$ ]] || exit 1
          ordinal=${BASH_REMATCH[1]}
          [[ $ordinal -eq 0 ]] && exit 0
          ncat --recv-only mysql-$(($ordinal-1)).mysql 3307 | xbstream -x -C /var/lib/mysql
          xtrabackup --prepare --target-dir=/var/lib/mysql`,
	}
}

func newFrameContainers(de *dataflowv1.DataflowEngine) []corev1.Container {

	return []corev1.Container{
		{
			Name:            "mysql",
			Image:           de.Spec.FrameStandalone.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports: []corev1.ContainerPort{
				{
					Name:          "mysql",
					ContainerPort: CONTAINER_PORT,
				},
			},
			Env: []corev1.EnvVar{
				{
					Name:  "MYSQL_ALLOW_EMPTY_PASSWORD",
					Value: "1",
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "data",
					MountPath: "/var/lib/mysql",
					SubPath:   "mysql",
				},
				{
					Name:      "conf",
					MountPath: "/etc/mysql/conf.d",
				},
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"mysqladmin", "ping",
						},
					},
				},
				InitialDelaySeconds: 10,
				PeriodSeconds:       600,
				TimeoutSeconds:      60,
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"mysql",
							"-h",
							"127.0.0.1",
							"-e",
							"SELECT 1",
						},
					},
				},
				InitialDelaySeconds: 5,
				PeriodSeconds:       2,
				TimeoutSeconds:      1,
			},
		},
		{
			Name:  "xtrabackup",
			Image: "gcr.io/google-samples/xtrabackup:1.0",
			Ports: []corev1.ContainerPort{
				{
					Name:          "xtrabackup",
					ContainerPort: 3307,
				},
			},
			Command: de.Spec.FrameStandalone.BackupCommand,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "data",
					MountPath: "/var/lib/mysql",
					SubPath:   "mysql",
				},
				{
					Name:      "conf",
					MountPath: "/etc/mysql/conf.d",
				},
			},
		},
	}
}

func loadBackupCommand() []string {
	return []string{
		"bash", "-c",
		`set -ex
        cd /var/lib/mysql

        # 确定克隆数据的 binlog 位置（如果有的话）。
        if [[ -f xtrabackup_slave_info && "x$(<xtrabackup_slave_info)" != "x" ]]; then
          # XtraBackup 已经生成了部分的 “CHANGE MASTER TO” 查询
          # 因为我们从一个现有副本进行克隆。(需要删除末尾的分号!)
          cat xtrabackup_slave_info | sed -E 's/;$//g' > change_master_to.sql.in
          # 在这里要忽略 xtrabackup_binlog_info （它是没用的）。
          rm -f xtrabackup_slave_info xtrabackup_binlog_info
        elif [[ -f xtrabackup_binlog_info ]]; then
          # 我们直接从主实例进行克隆。解析 binlog 位置。
          [[ echo $(cat xtrabackup_binlog_info) =~ ^(.*?)[[:space:]]+(.*?)$ ]] || exit 1
          rm -f xtrabackup_binlog_info xtrabackup_slave_info
          echo "CHANGE MASTER TO MASTER_LOG_FILE='${BASH_REMATCH[1]}',\
                MASTER_LOG_POS=${BASH_REMATCH[2]}" > change_master_to.sql.in
        fi

        # 检查我们是否需要通过启动复制来完成克隆。
        if [[ -f change_master_to.sql.in ]]; then
          echo "Waiting for mysqld to be ready (accepting connections)"
          until mysql -h 127.0.0.1 -e "SELECT 1"; do sleep 1; done

          echo "Initializing replication from clone position"
          mysql -h 127.0.0.1 \
                -e "$(<change_master_to.sql.in), \
                        MASTER_HOST='mysql-0.mysql', \
                        MASTER_USER='root', \
                        MASTER_PASSWORD='', \
                        MASTER_CONNECT_RETRY=10; \
                      START SLAVE;" || exit 1
          # 如果容器重新启动，最多尝试一次。
          mv change_master_to.sql.in change_master_to.sql.orig
        fi

        # 当对等点请求时，启动服务器发送备份。
        exec ncat --listen --keep-open --send-only --max-conns=1 3307 -c \
          "xtrabackup --backup --slave-info --stream=xbstream --host=127.0.0.1 --user=root"`,
	}
}
