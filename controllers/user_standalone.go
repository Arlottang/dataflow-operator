package controllers

import (
	"context"
	dataflowv1 "github.com/StepOnce7/dataflow-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"
)

func (r *DataflowEngineReconciler) ReconcileUserStandalone(ctx context.Context, instance *dataflowv1.DataflowEngine, req ctrl.Request) (ctrl.Result, error) {

	logg := log.FromContext(ctx)

	statefulSet := &appsv1.StatefulSet{}

	logg.Info("5 find etcd statefulSet")
	err := r.Get(ctx, req.NamespacedName, statefulSet)
	if err != nil {
		if errors.IsNotFound(err) {
			logg.Info("5.1 etcd statefulSet is not exists")

			if err = MutateHeadlessSvc(ctx, r, instance, req); err != nil {
				logg.Error(err, "5.2 create etcd service error")
				return ctrl.Result{}, err
			}

			if err = MutateStatefulSet(ctx, r, instance, req); err != nil {
				logg.Error(err, "5.3 create etcd statefulSet error")
				return ctrl.Result{}, err
			}

			logg.Info("5.4 create statefulSet success")
		} else {
			logg.Error(err, "5.5 get etcd statefulSet error")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// MutateStatefulSet is etcd cluster for user standalone
func MutateStatefulSet(ctx context.Context, r *DataflowEngineReconciler, de *dataflowv1.DataflowEngine, req ctrl.Request) error {

	logg := log.FromContext(ctx)

	logg.Info("start create statefulSet for etcd")

	sts := &appsv1.StatefulSet{}

	// todo: add label find diff statefulSet
	err := r.Get(ctx, req.NamespacedName, sts)
	if err == nil {
		logg.Info("d.2 etcd statefulSet exists")
		return nil
	}

	if !errors.IsNotFound(err) {
		logg.Error(err, "d.3 query etcd statefulSet error")
		return err
	}

	str := "etcd"
	sts = &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: de.Namespace,
			Name:      USER_STANDALONE,
			Labels: map[string]string{
				"storage": USER_STANDALONE,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    de.Spec.UserStandalone.Size,
			ServiceName: USER_STANDALONE,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"storage": USER_STANDALONE,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: de.Namespace,
					Labels: map[string]string{
						"storage":                 USER_STANDALONE,
						EtcdClusterCommonLabelKey: "etcd",
					},
				},
				Spec: corev1.PodSpec{
					Containers: newContainers(de),
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: de.Namespace,
						Name:      "etcd-persistent-storage",
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
						StorageClassName: &str,
					},
				},
			},
		},
	}

	logg.Info("set etcd statefulSet reference")
	if err = controllerutil.SetControllerReference(de, sts, r.Scheme); err != nil {
		logg.Error(err, "set etcd StatefulSet reference error")
		return err
	}

	logg.Info("d.6 start etcd statefulSet")
	if err = r.Create(ctx, sts); err != nil {
		logg.Error(err, "d.7 create etcd StatefulSet error")
		return err
	}

	logg.Info("d.8 create etcd StatefulSet success")
	return nil
}

func newContainers(de *dataflowv1.DataflowEngine) []corev1.Container {
	return []corev1.Container{
		{
			Name:  "etcd",
			Image: de.Spec.UserStandalone.Image,
			Ports: []corev1.ContainerPort{
				{
					Name:          "peer",
					ContainerPort: de.Spec.UserStandalone.Ports[1],
				}, {
					Name:          "client",
					ContainerPort: de.Spec.UserStandalone.Ports[0],
				},
			},
			Env: []corev1.EnvVar{
				{
					Name:  "INITIAL_CLUSTER_SIZE",
					Value: strconv.Itoa(int(*de.Spec.UserStandalone.Size)),
				},
				{
					Name:  "SET_NAME",
					Value: de.Spec.UserStandalone.Name,
				},
				{
					Name: "POD_IP",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "status.podIP",
						},
					},
				},
				{
					Name: "MY_NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.namespace",
						},
					},
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "etcd-persistent-storage",
					MountPath: "/var/run/etcd",
				},
			},
			// todo: need see doc
			Command: loadCommand(),
			Lifecycle: &corev1.Lifecycle{
				PreStop: &corev1.LifecycleHandler{
					Exec: &corev1.ExecAction{
						// todo: need see doc
						Command: loadActionCommand(),
					},
				},
			},
		},
	}
}

func loadCommand() []string {
	res := []string{
		"/bin/sh", "-ec",
		"HOSTNAME=$(hostname)\n\n              ETCDCTL_API=3\n\n              eps() {\n                  EPS=\"\"\n                  for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do\n                      EPS=\"${EPS}${EPS:+,}http://${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379\"\n                  done\n                  echo ${EPS}\n              }\n\n              member_hash() {\n                  etcdctl member list | grep -w \"$HOSTNAME\" | awk '{ print $1}' | awk -F \",\" '{ print $1}'\n              }\n\n              initial_peers() {\n                  PEERS=\"\"\n                  for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do\n                    PEERS=\"${PEERS}${PEERS:+,}${SET_NAME}-${i}=http://${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380\"\n                  done\n                  echo ${PEERS}\n              }\n\n              # etcd-SET_ID\n              SET_ID=${HOSTNAME##*-}\n\n              # adding a new member to existing cluster (assuming all initial pods are available)\n              if [ \"${SET_ID}\" -ge ${INITIAL_CLUSTER_SIZE} ]; then\n                  # export ETCDCTL_ENDPOINTS=$(eps)\n                  # member already added?\n\n                  MEMBER_HASH=$(member_hash)\n                  if [ -n \"${MEMBER_HASH}\" ]; then\n                      # the member hash exists but for some reason etcd failed\n                      # as the datadir has not be created, we can remove the member\n                      # and retrieve new hash\n                      echo \"Remove member ${MEMBER_HASH}\"\n                      etcdctl --endpoints=$(eps) member remove ${MEMBER_HASH}\n                  fi\n\n                  echo \"Adding new member\"\n\n                  etcdctl member --endpoints=$(eps) add ${HOSTNAME} --peer-urls=http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380 | grep \"^ETCD_\" > /var/run/etcd/new_member_envs\n\n                  if [ $? -ne 0 ]; then\n                      echo \"member add ${HOSTNAME} error.\"\n                      rm -f /var/run/etcd/new_member_envs\n                      exit 1\n                  fi\n\n                  echo \"==> Loading env vars of existing cluster...\"\n                  sed -ie \"s/^/export /\" /var/run/etcd/new_member_envs\n                  cat /var/run/etcd/new_member_envs\n                  . /var/run/etcd/new_member_envs\n\n                  exec etcd --listen-peer-urls http://${POD_IP}:2380 \\\n                      --listen-client-urls http://${POD_IP}:2379,http://127.0.0.1:2379 \\\n                      --advertise-client-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379 \\\n                      --data-dir /var/run/etcd/default.etcd\n              fi\n\n              for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do\n                  while true; do\n                      echo \"Waiting for ${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local to come up\"\n                      ping -W 1 -c 1 ${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local > /dev/null && break\n                      sleep 1s\n                  done\n              done\n\n              echo \"join member ${HOSTNAME}\"\n              # join member\n              exec etcd --name ${HOSTNAME} \\\n                  --initial-advertise-peer-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380 \\\n                  --listen-peer-urls http://${POD_IP}:2380 \\\n                  --listen-client-urls http://${POD_IP}:2379,http://127.0.0.1:2379 \\\n                  --advertise-client-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379 \\\n                  --initial-cluster-token etcd-cluster-1 \\\n                  --data-dir /var/run/etcd/default.etcd \\\n                  --initial-cluster $(initial_peers) \\\n                  --initial-cluster-state new",
	}
	return res
}

func loadActionCommand() []string {
	res := []string{
		"/bin/sh", "-ec",
		"HOSTNAME=$(hostname)\n\n                    member_hash() {\n                        etcdctl member list | grep -w \"$HOSTNAME\" | awk '{ print $1}' | awk -F \",\" '{ print $1}'\n                    }\n\n                    eps() {\n                        EPS=\"\"\n                        for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do\n                            EPS=\"${EPS}${EPS:+,}http://${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379\"\n                        done\n                        echo ${EPS}\n                    }\n\n                    export ETCDCTL_ENDPOINTS=$(eps)\n                    SET_ID=${HOSTNAME##*-}\n\n                    # Removing member from cluster\n                    if [ \"${SET_ID}\" -ge ${INITIAL_CLUSTER_SIZE} ]; then\n                        echo \"Removing ${HOSTNAME} from etcd cluster\"\n                        etcdctl member remove $(member_hash)\n                        if [ $? -eq 0 ]; then\n                            # Remove everything otherwise the cluster will no longer scale-up\n                            rm -rf /var/run/etcd/*\n                        fi\n                    fi",
	}
	return res
}

func MutateHeadlessSvc(ctx context.Context, r *DataflowEngineReconciler, de *dataflowv1.DataflowEngine, req ctrl.Request) error {

	logg := log.FromContext(ctx)

	logg.Info("start create etcd service if not exists")

	svc := &corev1.Service{}

	// todo: add label find diff service
	err := r.Get(ctx, req.NamespacedName, svc)
	if err == nil {
		logg.Info(" etcd service exists")
		return nil
	}

	if !errors.IsNotFound(err) {
		logg.Error(err, " query etcd service error")
		return err
	}

	svc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: de.Namespace,
			Name:      "etcd-service",
			Labels: map[string]string{
				EtcdClusterCommonLabelKey: "etcd",
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector: map[string]string{
				"storage": USER_STANDALONE,
			},
			Ports: []corev1.ServicePort{
				{
					Name: "peer",
					Port: de.Spec.UserStandalone.Ports[1],
				},
				{
					Name: "client",
					Port: de.Spec.UserStandalone.Ports[0],
				},
			},
		},
	}
	logg.Info("c.4 set etcd service reference")

	if err = controllerutil.SetControllerReference(de, svc, r.Scheme); err != nil {
		logg.Error(err, "c.5 set etcd service reference error")
		return err
	}

	logg.Info("c.6 start create etcd service")

	if err = r.Create(ctx, svc); err != nil {
		logg.Error(err, "c.7 create etcd service error")
		return err
	}

	logg.Info("c.8 create etcd service success")
	return nil
}
