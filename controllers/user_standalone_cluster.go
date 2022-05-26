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
	"strconv"
)

func (r *DataflowEngineReconciler) ReconcileEtcdCluster(ctx context.Context, instance *dataflowv1.DataflowEngine, req ctrl.Request) (ctrl.Result, error) {

	logg := log.FromContext(ctx)

	var svc corev1.Service
	svc.Name = instance.Spec.UserStandalone.Name
	svc.Namespace = instance.Namespace

	or, err := ctrl.CreateOrUpdate(ctx, r.Client, &svc, func() error {
		MutateHeadlessSvc(instance, &svc)
		return controllerutil.SetControllerReference(instance, &svc, r.Scheme)
	})

	if err != nil {
		logg.Error(err, "create etcd service error")
		return ctrl.Result{}, err
	}

	logg.Info("CreateOrUpdate", "Etcd Service", or)

	var sts appsv1.StatefulSet
	sts.Name = instance.Spec.UserStandalone.Name
	sts.Namespace = instance.Namespace
	or, err = ctrl.CreateOrUpdate(ctx, r.Client, &sts, func() error {
		MutateStatefulSet(instance, &sts)
		return ctrl.SetControllerReference(instance, &sts, r.Scheme)
	})
	if err != nil {
		logg.Error(err, "create etcd statefulSet error")
		return ctrl.Result{}, err
	}

	logg.Info("CreateOrUpdate", "Etcd StatefulSet", or)

	logg.Info("user standalone reconcile end", "reconcile", "success")
	return ctrl.Result{}, nil
}

// MutateStatefulSet is etcd cluster for user standalone
func MutateStatefulSet(de *dataflowv1.DataflowEngine, sts *appsv1.StatefulSet) {

	sts.Labels = map[string]string{
		EtcdClusterCommonLabelKey: "etcd",
	}

	sts.Spec = appsv1.StatefulSetSpec{
		Replicas: de.Spec.UserStandalone.Size,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				EtcdClusterLabelKey: de.Spec.UserStandalone.Name,
			},
		},
		ServiceName: de.Spec.UserStandalone.Name,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					EtcdClusterCommonLabelKey: "etcd",
					EtcdClusterLabelKey:       de.Spec.UserStandalone.Name,
				},
			},
			Spec: corev1.PodSpec{
				Containers: newContainers(de),
			},
		},
		VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: EtcdDataDirName,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1G"),
						},
					},
				},
			},
		},
	}

}

func newContainers(de *dataflowv1.DataflowEngine) []corev1.Container {
	return []corev1.Container{
		{
			Name:            "etcd",
			Image:           de.Spec.UserStandalone.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports: []corev1.ContainerPort{
				{
					Name:          "peer",
					ContainerPort: 2380,
					Protocol:      corev1.ProtocolTCP,
				}, {
					Name:          "client",
					ContainerPort: 2379,
					Protocol:      corev1.ProtocolTCP,
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
			Command: loadCommand(),
			Lifecycle: &corev1.Lifecycle{
				PreStop: &corev1.LifecycleHandler{
					Exec: &corev1.ExecAction{
						// todo: need see doc
						Command: loadActionCommand(),
					},
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      EtcdDataDirName,
					MountPath: "/var/run/etcd",
				},
			},
		},
	}
}

func loadCommand() []string {
	res := []string{

		"/bin/sh", "-ec",
		`HOSTNAME=$(hostname)

              ETCDCTL_API=3

              eps() {
                  EPS=""
                  for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do
                      EPS="${EPS}${EPS:+,}http://${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379"
                  done
                  echo ${EPS}
              }

              member_hash() {
                  etcdctl member list | grep -w "$HOSTNAME" | awk '{ print $1}' | awk -F "," '{ print $1}'
              }

              initial_peers() {
                  PEERS=""
                  for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do
                    PEERS="${PEERS}${PEERS:+,}${SET_NAME}-${i}=http://${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380"
                  done
                  echo ${PEERS}
              }

              # etcd-SET_ID
              SET_ID=${HOSTNAME##*-}

              # adding a new member to existing cluster (assuming all initial pods are available)
              if [ "${SET_ID}" -ge ${INITIAL_CLUSTER_SIZE} ]; then
                  # export ETCDCTL_ENDPOINTS=$(eps)
                  # member already added?

                  MEMBER_HASH=$(member_hash)
                  if [ -n "${MEMBER_HASH}" ]; then
                      # the member hash exists but for some reason etcd failed
                      # as the datadir has not be created, we can remove the member
                      # and retrieve new hash
                      echo "Remove member ${MEMBER_HASH}"
                      etcdctl --endpoints=$(eps) member remove ${MEMBER_HASH}
                  fi

                  echo "Adding new member"

                  echo "etcdctl --endpoints=$(eps) member add ${HOSTNAME} --peer-urls=http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380"
                  etcdctl member --endpoints=$(eps) add ${HOSTNAME} --peer-urls=http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380 | grep "^ETCD_" > /var/run/etcd/new_member_envs

                  if [ $? -ne 0 ]; then
                      echo "member add ${HOSTNAME} error."
                      rm -f /var/run/etcd/new_member_envs
                      exit 1
                  fi

                  echo "==> Loading env vars of existing cluster..."
                  sed -ie "s/^/export /" /var/run/etcd/new_member_envs
                  cat /var/run/etcd/new_member_envs
                  . /var/run/etcd/new_member_envs

                  echo "etcd --name ${HOSTNAME} --initial-advertise-peer-urls ${ETCD_INITIAL_ADVERTISE_PEER_URLS} --listen-peer-urls http://${POD_IP}:2380 --listen-client-urls http://${POD_IP}:2379,http://127.0.0.1:2379 --advertise-client-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379 --data-dir /var/run/etcd/default.etcd --initial-cluster ${ETCD_INITIAL_CLUSTER} --initial-cluster-state ${ETCD_INITIAL_CLUSTER_STATE}"

                  exec etcd --listen-peer-urls http://${POD_IP}:2380 \
                      --listen-client-urls http://${POD_IP}:2379,http://127.0.0.1:2379 \
                      --advertise-client-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379 \
                      --data-dir /var/run/etcd/default.etcd
              fi

              for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do
                  while true; do
                      echo "Waiting for ${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local to come up"
                      ping -W 1 -c 1 ${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local > /dev/null && break
                      sleep 1s
                  done
              done

              echo "join member ${HOSTNAME}"
              # join member
              exec etcd --name ${HOSTNAME} \
                  --initial-advertise-peer-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2380 \
                  --listen-peer-urls http://${POD_IP}:2380 \
                  --listen-client-urls http://${POD_IP}:2379,http://127.0.0.1:2379 \
                  --advertise-client-urls http://${HOSTNAME}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379 \
                  --initial-cluster-token etcd-cluster-1 \
                  --data-dir /var/run/etcd/default.etcd \
                  --initial-cluster $(initial_peers) \
                  --initial-cluster-state new`,
	}

	return res
}

func loadActionCommand() []string {
	res := []string{
		"/bin/sh", "-ec",
		`HOSTNAME=$(hostname)

                    member_hash() {
                        etcdctl member list | grep -w "$HOSTNAME" | awk '{ print $1}' | awk -F "," '{ print $1}'
                    }

                    eps() {
                        EPS=""
                        for i in $(seq 0 $((${INITIAL_CLUSTER_SIZE} - 1))); do
                            EPS="${EPS}${EPS:+,}http://${SET_NAME}-${i}.${SET_NAME}.${MY_NAMESPACE}.svc.cluster.local:2379"
                        done
                        echo ${EPS}
                    }

                    export ETCDCTL_ENDPOINTS=$(eps)
                    SET_ID=${HOSTNAME##*-}

                    # Removing member from cluster
                    if [ "${SET_ID}" -ge ${INITIAL_CLUSTER_SIZE} ]; then
                        echo "Removing ${HOSTNAME} from etcd cluster"
                        etcdctl member remove $(member_hash)
                        if [ $? -eq 0 ]; then
                            # Remove everything otherwise the cluster will no longer scale-up
                            rm -rf /var/run/etcd/*
                        fi
                    fi`,
	}
	return res
}

func MutateHeadlessSvc(de *dataflowv1.DataflowEngine, svc *corev1.Service) {

	svc.Labels = map[string]string{
		EtcdClusterCommonLabelKey: "etcd",
	}

	svc.Spec = corev1.ServiceSpec{
		ClusterIP: corev1.ClusterIPNone,
		Selector: map[string]string{
			EtcdClusterLabelKey: de.Spec.UserStandalone.Name,
		},

		// todo: will be change soon
		Ports: []corev1.ServicePort{
			{
				Name:     "peer",
				Port:     2380,
				Protocol: corev1.ProtocolTCP,
			},
			{
				Name:     "client",
				Port:     2379,
				Protocol: corev1.ProtocolTCP,
			},
		},
	}
}
